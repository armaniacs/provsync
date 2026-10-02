package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMarshalIndentSortedSortsKeys(t *testing.T) {
	got, err := MarshalIndentSorted(map[string]any{"b": 1, "a": 2})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := "{\n  \"a\": 2,\n  \"b\": 1\n}"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteFileAtomicPreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new")); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte("new")) {
		t.Errorf("content = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestWriteFileAtomicCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "f.json")
	if err := WriteFileAtomic(path, []byte("{}\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestWriteFileAtomicNewFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.json")
	if err := WriteFileAtomic(path, []byte("{}\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("new file mode = %o, want 600", info.Mode().Perm())
	}
	sub := filepath.Join(dir, "sub")
	path2 := filepath.Join(sub, "f.json")
	if err := WriteFileAtomic(path2, []byte("{}\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err = os.Stat(sub)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("new dir mode = %o, want 700", info.Mode().Perm())
	}
}

func TestWriteFileAtomicKeepsExistingDirMode(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(filepath.Join(sub, "f.json"), []byte("{}\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(sub)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("existing dir mode = %o, want 755", info.Mode().Perm())
	}
}
