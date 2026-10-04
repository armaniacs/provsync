// Package cli のうち、provsync 管理ファイルの完全削除を担う。
package cli

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/lock"
)

// cleanupTargets は削除対象の実在パスを返す(セントラル設定・状態ディレクトリ)。
func cleanupTargets(root adapter.Root) []string {
	var out []string
	if central := root.CentralConfigPath(); statExists(central) {
		out = append(out, central)
	}
	if st := root.StateDir(); isDir(st) {
		out = append(out, st)
	}
	return out
}

func statExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// trashOne moves path to the Trash on darwin and deletes it elsewhere.
// It returns trashed=true when the file went to the Trash.
// The useTrash flag exists so tests can exercise both paths on any OS.
func trashOne(path string, useTrash bool) (bool, error) {
	if !useTrash {
		return false, os.RemoveAll(path)
	}
	// Finder resolves the path itself; escape backslashes before double quotes
	// so AppleScript does not misinterpret either.
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	cmd := exec.Command("osascript", "-e",
		`tell application "Finder" to delete POSIX file "`+escaped+`"`)
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return true, nil
}

// dirSize は配下ファイルの合計バイト数を返す。読めない項目は飛ばす
// (削除時のエラーをここで先取りしない)。
func dirSize(path string) int64 {
	var n int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func formatSize(n int64) string {
	if n < 0 {
		n = 0
	}
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// parseConfirm は確認入力の肯定判定。y / yes のみ真(前後空白・大小無視)。
func parseConfirm(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}

// confirmFrom は確認入力を 1 行読んで判定する。空入力(EOF)は中止 (false, nil)。
func confirmFrom(r io.Reader) (bool, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	if line == "" {
		return false, nil
	}
	return parseConfirm(line), nil
}

// isTerminalStdin は stdin が端末かどうかを返す。単体テストは環境依存のため置かない。
func isTerminalStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// askConfirm は対話確認を行う。非端末 stdin では err.cleanup.interactive を返す。
// プロンプトは stdout を汚さないよう errOut へ出す。
func askConfirm(o *options) (bool, error) {
	if !isTerminalStdin() {
		return false, i18n.New("err.cleanup.interactive")
	}
	fmt.Fprintf(o.errOut, "%s ", o.T("msg.cleanup.confirm"))
	return confirmFrom(os.Stdin)
}

// cmdCleanup は provsync 管理ファイルの完全削除を行う。
// プレビューが既定で、--write は対話確認のうえ実行、--yes は確認なしで実行する。
func cmdCleanup(o *options, args []string) error {
	if len(args) > 0 {
		return o.usageErr("err.usage.cleanup")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	targets := cleanupTargets(root)
	if len(targets) == 0 {
		o.msgf(o.out, "msg.cleanup.none")
		return nil
	}
	o.msgf(o.out, "msg.cleanup.preview")
	var total int64
	for _, p := range targets {
		sz := dirSize(p)
		total += sz
		o.msgf(o.out, "msg.cleanup.target", p, formatSize(sz))
	}
	o.msgf(o.out, "msg.cleanup.total", formatSize(total))
	if !o.write && !o.yes {
		return nil
	}
	if !o.yes {
		ok, err := askConfirm(o)
		if err != nil {
			return err
		}
		if !ok {
			o.msgf(o.out, "msg.cleanup.cancelled")
			return nil
		}
	}
	release, err := lock.Acquire(root.StateDir(), 10*time.Second)
	if err != nil {
		return err
	}
	defer release()
	// lock.Acquire が状態ディレクトリを作るため、ここで数え直す。
	// セントラルのみの環境でも空の状態ディレクトリを残さない。
	targets = cleanupTargets(root)
	useTrash := runtime.GOOS == "darwin"
	trash := o.trash
	if trash == nil {
		trash = func(p string) (bool, error) { return trashOne(p, runtime.GOOS == "darwin") }
	}
	var firstErr error
	for _, p := range targets {
		if useTrash || o.trash != nil {
			if _, err := trash(p); err != nil {
				o.warnf("warn.cleanup.trashFallback", p, err)
			} else {
				o.msgf(o.out, "msg.cleanup.trashed", p)
				continue
			}
		}
		if err := os.RemoveAll(p); err != nil {
			o.warnf("warn.cleanup.failed", p)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		o.msgf(o.out, "msg.cleanup.deleted", p)
	}
	return firstErr
}
