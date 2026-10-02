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

func TestWriteFileAtomicUpdatesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	if err := os.WriteFile(real, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("new")); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink was replaced by a regular file")
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("target content = %q, want new", data)
	}
}

func TestWriteFileAtomicRelativeSymlink(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "real.json"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink("sub/real.json", link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("new")); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(sub, "real.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("target content = %q, want new", data)
	}
}

func TestWriteFileAtomicMultiHopSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	if err := os.WriteFile(real, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link1 := filepath.Join(dir, "link1.json")
	if err := os.Symlink(real, link1); err != nil {
		t.Fatal(err)
	}
	link2 := filepath.Join(dir, "link2.json")
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link2, []byte("new")); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Lstat(link1)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("intermediate symlink was replaced by a regular file")
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("target content = %q, want new", data)
	}
}

func TestWriteFileAtomicBrokenSymlinkErrors(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(filepath.Join(dir, "missing.json"), link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("new")); err == nil {
		t.Error("expected error for broken symlink")
	}
}
