// Package backup はタイムスタンプ付きバックアップ、マニフェスト、復元を担う。
package backup

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/i18n"
)

// MaxOperations は保持する操作履歴の最大数。
const MaxOperations = 20

// FileRef は 1 ファイルのバックアップ参照。
type FileRef struct {
	Path       string      `json:"path"`
	BackupPath string      `json:"backupPath,omitempty"`
	Mode       os.FileMode `json:"mode"`
	SHA256     string      `json:"sha256,omitempty"`
	Existed    bool        `json:"existed"`
}

// Operation は 1 回の書き込み操作の記録。
type Operation struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"startedAt"`
	Command   string    `json:"command"`
	Files     []FileRef `json:"files"`
	Undone    bool      `json:"undone,omitempty"`
	// NoBackup は --no-backup での書き込みを示すマーカー。
	// Files が空で、undo の対象にはならない。
	NoBackup bool `json:"noBackup,omitempty"`
}

type index struct {
	Operations []Operation `json:"operations"`
}

// Store は状態ディレクトリ配下のバックアップを管理する。
type Store struct {
	dir string
	max int
}

// New は状態ディレクトリを指定して Store を作る。
func New(stateDir string) *Store {
	return &Store{dir: stateDir, max: MaxOperations}
}

// SetMax は保持する操作履歴の最大数を変更する。
func (s *Store) SetMax(n int) {
	s.max = n
}

func (s *Store) indexPath() string  { return filepath.Join(s.dir, "index.json") }
func (s *Store) backupsDir() string { return filepath.Join(s.dir, "backups") }

func (s *Store) load() (*index, error) {
	raw, err := os.ReadFile(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &index{}, nil
		}
		return nil, err
	}
	var idx index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, i18n.Wrap(err, "err.manifest.invalid")
	}
	return &idx, nil
}

func (s *Store) save(idx *index) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.indexPath(), append(data, '\n'))
}

// appendAndSave prepends op to the history and persists it via prune+save.
// Record, RecordMarker and Restore share this single history-update path so a
// future retention policy change lands in one place.
func (s *Store) appendAndSave(idx *index, op Operation) error {
	idx.Operations = append([]Operation{op}, idx.Operations...)
	if err := s.prune(idx); err != nil {
		return err
	}
	return s.save(idx)
}

// Record は paths の現在の内容を新しい操作としてバックアップする。
func (s *Store) Record(command string, paths []string) (Operation, error) {
	op := Operation{
		ID:        newID(),
		StartedAt: time.Now().UTC(),
		Command:   command,
	}
	opDir := filepath.Join(s.backupsDir(), op.ID)
	if err := os.MkdirAll(opDir, 0o700); err != nil {
		return Operation{}, err
	}
	for i, p := range paths {
		ref, err := capture(p, opDir, i)
		if err != nil {
			// Best-effort cleanup so a failed record leaves no orphan
			// directory behind; the original error is returned.
			_ = os.RemoveAll(opDir)
			return Operation{}, err
		}
		op.Files = append(op.Files, ref)
	}

	idx, err := s.load()
	if err != nil {
		return Operation{}, err
	}
	if err := s.appendAndSave(idx, op); err != nil {
		return Operation{}, err
	}
	return op, nil
}

// RecordMarker はバックアップ無しで行われた書き込みを履歴に記録する。
// Files は空で、undo の対象にはならないが undo 時の警告と --list 表示に使う。
func (s *Store) RecordMarker(command string) (Operation, error) {
	op := Operation{
		ID:        newID(),
		StartedAt: time.Now().UTC(),
		Command:   command,
		NoBackup:  true,
	}
	idx, err := s.load()
	if err != nil {
		return Operation{}, err
	}
	if err := s.appendAndSave(idx, op); err != nil {
		return Operation{}, err
	}
	return op, nil
}

// List は操作履歴(新しい順)を返す。
func (s *Store) List() ([]Operation, error) {
	idx, err := s.load()
	if err != nil {
		return nil, err
	}
	return idx.Operations, nil
}

// Find は ID で操作を探す。id が空なら未 undo の最新操作を返す
// (バックアップ無しのマーカー操作は対象外)。
func (s *Store) Find(id string) (Operation, error) {
	ops, err := s.List()
	if err != nil {
		return Operation{}, err
	}
	if id == "" {
		for _, op := range ops {
			if !op.Undone && !op.NoBackup {
				return op, nil
			}
		}
		return Operation{}, i18n.New("err.undo.none")
	}
	for _, op := range ops {
		if op.ID == id {
			return op, nil
		}
	}
	return Operation{}, i18n.New("err.undo.notFound", id)
}

// Restore は指定操作を復元する。復元前の現状を新しい操作として記録し、
// 元操作を undone にする(undo 自体を undo できる)。
// redo 用の記録を先にマニフェストへ反映してから復元するため、
// 復元の途中で失敗しても redo は常に index から到達可能である。
func (s *Store) Restore(op Operation) (Operation, error) {
	idx, err := s.load()
	if err != nil {
		return Operation{}, err
	}

	undoOp := Operation{
		ID:        newID(),
		StartedAt: time.Now().UTC(),
		Command:   "undo " + op.ID,
	}
	undoDir := filepath.Join(s.backupsDir(), undoOp.ID)
	if err := os.MkdirAll(undoDir, 0o700); err != nil {
		return Operation{}, err
	}
	for i, f := range op.Files {
		ref, err := capture(f.Path, undoDir, i)
		if err != nil {
			return Operation{}, err
		}
		undoOp.Files = append(undoOp.Files, ref)
	}

	if err := s.appendAndSave(idx, undoOp); err != nil {
		return Operation{}, err
	}

	for _, f := range op.Files {
		if err := restoreFile(f); err != nil {
			return undoOp, err
		}
	}

	for i := range idx.Operations {
		if idx.Operations[i].ID == op.ID {
			idx.Operations[i].Undone = true
		}
	}
	if err := s.save(idx); err != nil {
		return undoOp, err
	}
	return undoOp, nil
}

func (s *Store) prune(idx *index) error {
	if len(idx.Operations) <= s.max {
		return nil
	}
	return s.removeBeyond(idx, s.max)
}

// Prune は新しい keep 件を残して残りの操作を履歴と実体の両方から削除する。
// 削除件数を返す。
func (s *Store) Prune(keep int) (int, error) {
	idx, err := s.load()
	if err != nil {
		return 0, err
	}
	if keep < 0 {
		keep = 0
	}
	if len(idx.Operations) <= keep {
		return 0, nil
	}
	removed := len(idx.Operations) - keep
	if err := s.removeBeyond(idx, keep); err != nil {
		return 0, err
	}
	if err := s.save(idx); err != nil {
		return 0, err
	}
	return removed, nil
}

// removeBeyond drops history entries beyond keep and deletes their
// backup directories. All retention paths share this helper.
func (s *Store) removeBeyond(idx *index, keep int) error {
	removed := idx.Operations[keep:]
	idx.Operations = idx.Operations[:keep]
	for _, op := range removed {
		if err := os.RemoveAll(filepath.Join(s.backupsDir(), op.ID)); err != nil {
			return err
		}
	}
	return nil
}

func capture(path, opDir string, seq int) (FileRef, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileRef{Path: path, Existed: false}, nil
		}
		return FileRef{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return FileRef{}, err
	}
	sum := sha256.Sum256(data)
	backupPath := filepath.Join(opDir, fmt.Sprintf("%02d-%s", seq, filepath.Base(path)))
	if err := os.WriteFile(backupPath, data, info.Mode().Perm()); err != nil {
		return FileRef{}, err
	}
	return FileRef{
		Path:       path,
		BackupPath: backupPath,
		Mode:       info.Mode().Perm(),
		SHA256:     hex.EncodeToString(sum[:]),
		Existed:    true,
	}, nil
}

func restoreFile(f FileRef) error {
	if !f.Existed {
		if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := os.ReadFile(f.BackupPath)
	if err != nil {
		return i18n.Wrap(err, "err.backup.read", f.BackupPath)
	}
	sum := sha256.Sum256(data)
	if f.SHA256 != "" && hex.EncodeToString(sum[:]) != f.SHA256 {
		return i18n.New("err.backup.hashMismatch", f.BackupPath)
	}
	if err := fsutil.WriteFileAtomic(f.Path, data); err != nil {
		return err
	}
	return os.Chmod(f.Path, f.Mode)
}

func newID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return time.Now().UTC().Format("20060102T150405")
	}
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(buf[:])
}
