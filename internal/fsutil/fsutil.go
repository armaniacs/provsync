// Package fsutil は JSON 直列化とアトミックなファイル I/O を提供する。
package fsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
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
// 既存ファイルがある場合はそのパーミッションを引き継ぐ。
// 途中で失敗しても既存ファイルは壊れない。
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
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

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
