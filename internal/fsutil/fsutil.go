// Package fsutil は JSON 直列化とアトミックなファイル I/O を提供する。
package fsutil

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/armaniacs/provsync/internal/i18n"
)

// MarshalIndentSorted は v を JSON 化し、map 経由でキーをソートして
// 2 スペースインデントで整形する。構造体・map のどちらにも使える。
func MarshalIndentSorted(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tmp any
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, err
	}
	return json.MarshalIndent(tmp, "", "  ")
}

// WriteFileAtomic は一時ファイルへ書いてから rename で置換する。
// path がシンボリックリンクの場合はリンクを維持したままリンク先の実体を置換する
// (一時ファイルは必ずリンク先と同じディレクトリに作るため、rename が別
// ファイルシステムを跨がない)。既存ファイルがある場合はそのパーミッションを
// 引き継ぎ、新規ファイルは 0600、新規ディレクトリは 0700 で作る。
// 途中で失敗しても既存ファイルは壊れない。
func WriteFileAtomic(path string, data []byte) error {
	resolved, err := resolveWritePath(path)
	if err != nil {
		return err
	}
	path = resolved
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".provsync-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// resolveWritePath はシンボリックリンクを辿った実体のパスを返す。
// 存在しないパスと通常ファイルはそのまま返す。リンク切れはエラーにする。
func resolveWritePath(path string) (string, error) {
	li, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	if li.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", i18n.Wrap(err, "err.symlink.resolve", path)
	}
	return resolved, nil
}
