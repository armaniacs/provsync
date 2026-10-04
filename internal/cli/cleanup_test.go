package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashOneDirectDelete(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gone.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	trashed, err := trashOne(p, false)
	if err != nil {
		t.Fatalf("trashOne direct: %v", err)
	}
	if trashed {
		t.Error("direct delete must report trashed=false")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("file must be gone: %v", err)
	}
}

func TestTrashOneDarwinSuccess(t *testing.T) {
	dir := t.TempDir()
	bindir := t.TempDir()
	log := filepath.Join(dir, "osascript.log")
	script := "#!/bin/sh\necho \"$@\" >> \"" + log + "\"\nexit 0\n"
	bin := filepath.Join(bindir, "osascript")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bindir+":"+os.Getenv("PATH"))
	p := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	trashed, err := trashOne(p, true)
	if err != nil {
		t.Fatalf("trashOne trash: %v", err)
	}
	if !trashed {
		t.Error("must report trashed=true")
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), p) {
		t.Errorf("osascript must receive the path, log:\n%s", raw)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("fake osascript moves nothing, file must remain: %v", err)
	}
}

func TestTrashOneDarwinFailure(t *testing.T) {
	dir := t.TempDir()
	bindir := t.TempDir()
	bin := filepath.Join(bindir, "osascript")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bindir) // real osascript unreachable; only the failing fake
	p := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := trashOne(p, true); err == nil {
		t.Error("failing osascript must return an error (caller falls back)")
	}
}

func TestFormatSize(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 512: "512 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 1048576: "1.0 MB"} {
		if got := formatSize(in); got != want {
			t.Errorf("formatSize(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseConfirm(t *testing.T) {
	for in, want := range map[string]bool{"y": true, "Y": true, "yes": true, " Yes ": true, "": false, "n": false, "no": false, "yep": false} {
		if got := parseConfirm(in); got != want {
			t.Errorf("parseConfirm(%q) = %v, want %v", in, got, want)
		}
	}
}
