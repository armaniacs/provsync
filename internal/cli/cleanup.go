// Package cli のうち、provsync 管理ファイルの完全削除を担う。
package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/i18n"
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

// askConfirm は対話確認を行う。非端末 stdin では err.cleanup.interactive を返す。
func askConfirm(o *options) (bool, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return false, i18n.New("err.cleanup.interactive")
	}
	fmt.Fprintf(o.out, "%s ", o.T("msg.cleanup.confirm"))
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, err
	}
	return parseConfirm(line), nil
}
