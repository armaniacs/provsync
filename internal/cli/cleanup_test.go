package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/armaniacs/provsync/internal/lock"
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

func TestTrashOneDarwinEscapesBackslash(t *testing.T) {
	dir := t.TempDir()
	bindir := t.TempDir()
	log := filepath.Join(dir, "osascript.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"" + log + "\"\nexit 0\n"
	bin := filepath.Join(bindir, "osascript")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bindir+":"+os.Getenv("PATH"))
	p := filepath.Join(dir, `a\b`, "c.txt")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := trashOne(p, true); err != nil {
		t.Fatalf("trashOne trash: %v", err)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `a\\b`) {
		t.Errorf("osascript must receive the backslash doubly escaped, log:\n%s", raw)
	}
	q := filepath.Join(dir, `a"b`, "c.txt")
	if err := os.MkdirAll(filepath.Dir(q), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(q, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := trashOne(q, true); err != nil {
		t.Fatalf("trashOne trash: %v", err)
	}
	raw, err = os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "a\\\"b") {
		t.Errorf("osascript must receive the double quote escaped, log:\n%s", raw)
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
	for in, want := range map[int64]string{0: "0 B", -5: "0 B", -2097152: "0 B", 512: "512 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 1048576: "1.0 MB"} {
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

func TestConfirmFrom(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want bool
	}{
		{"y", "y\n", true},
		{"Y", "Y\n", true},
		{"yes", "yes\n", true},
		{"spaced yes", " yes \n", true},
		{"n", "n\n", false},
		{"no", "no\n", false},
		{"empty line", "\n", false},
		{"eof", "", false},
	} {
		got, err := confirmFrom(strings.NewReader(c.in))
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: confirmFrom(%q) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
	sentinel := errors.New("read failed")
	got, err := confirmFrom(errReader{err: sentinel})
	if got {
		t.Error("reader error must return false")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("reader error must propagate, got %v", err)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestCleanupRejectsArgs(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "en")
	_, err := run(t, f.root, "cleanup", "extra")
	if err == nil {
		t.Error("cleanup with args must fail")
	}
	if !strings.Contains(err.Error(), "usage: provsync cleanup") {
		t.Errorf("must show usage:\n%v", err)
	}
}

func TestCleanupPreviewListsTargets(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	out, err := run(t, f.root, "cleanup")
	if err != nil {
		t.Fatalf("preview must succeed: %v", err)
	}
	if !strings.Contains(out, f.central) {
		t.Errorf("preview must list the central config:\n%s", out)
	}
	if _, err := os.Stat(f.central); err != nil {
		t.Errorf("preview must not delete anything: %v", err)
	}
}

func TestCleanupNothingToRemove(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "en")
	// setup creates tool configs only; the central config and state dir do
	// not exist until a --write run, so there is nothing to remove. Cleanup
	// must never touch the tool configs.
	out, err := run(t, f.root, "cleanup", "--yes")
	if err != nil {
		t.Fatalf("--yes with no targets must succeed: %v", err)
	}
	if !strings.Contains(out, "nothing to remove") {
		t.Errorf("must report nothing to remove:\n%s", out)
	}
	for _, p := range []string{f.kilo, f.opencode} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("tool config must remain untouched: %s: %v", p, err)
		}
	}
}

func TestCleanupYesRemovesTargets(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	out, err := run(t, f.root, "cleanup", "--yes")
	if err != nil {
		t.Fatalf("--yes must succeed: %v", err)
	}
	if _, err := os.Stat(f.central); !os.IsNotExist(err) {
		t.Errorf("central config must be gone: %v", err)
	}
	stateDir := filepath.Join(f.root, ".local", "state", "provsync")
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("state dir must be gone: %v", err)
	}
	if !strings.Contains(out, f.central) {
		t.Errorf("must report removed targets:\n%s", out)
	}
}

func TestCleanupRemovesCreatedStateDir(t *testing.T) {
	f := setup(t)
	// 直接セントラルだけを書く。pull を経ないので状態ディレクトリはまだ無い。
	write(t, f.central, "{}")
	stateDir := filepath.Join(f.root, ".local", "state", "provsync")
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("state dir must not exist before cleanup: %v", err)
	}
	if _, err := run(t, f.root, "cleanup", "--yes"); err != nil {
		t.Fatalf("cleanup --yes must succeed: %v", err)
	}
	if _, err := os.Stat(f.central); !os.IsNotExist(err) {
		t.Errorf("central config must be gone: %v", err)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("created state dir must be gone: %v", err)
	}
}

func TestCleanupFallbackOnTrashFailure(t *testing.T) {
	f := setup(t)
	write(t, f.central, "{}")
	var outBuf, errBuf bytes.Buffer
	o := &options{
		out:      &outBuf,
		errOut:   &errBuf,
		lang:     "en",
		rootFlag: f.root,
		write:    true,
		yes:      true,
		trash: func(string) (bool, error) {
			return false, errors.New("no Finder")
		},
	}
	if err := cmdCleanup(o, nil); err != nil {
		t.Fatalf("cleanup must succeed after fallback: %v", err)
	}
	if _, err := os.Stat(f.central); !os.IsNotExist(err) {
		t.Errorf("central config must be gone: %v", err)
	}
	stateDir := filepath.Join(f.root, ".local", "state", "provsync")
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("state dir must be gone: %v", err)
	}
	if !strings.Contains(errBuf.String(), "could not move to Trash") {
		t.Errorf("stderr must contain the fallback warning:\n%s", errBuf.String())
	}
	if !strings.Contains(outBuf.String(), "deleted: ") {
		t.Errorf("stdout must contain deleted lines:\n%s", outBuf.String())
	}
}

func TestCleanupLockBusy(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	stateDir := filepath.Join(f.root, ".local", "state", "provsync")
	release, err := lock.Acquire(stateDir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// cleanup はロックタイムアウト(約 10 秒)を待ち切ってから busy を返す。
	if _, err := run(t, f.root, "cleanup", "--yes"); err == nil {
		t.Fatal("cleanup must fail while the lock is held")
	}
	if _, err := os.Stat(f.central); err != nil {
		t.Errorf("central config must remain: %v", err)
	}
	if _, err := os.Stat(stateDir); err != nil {
		t.Errorf("state dir must remain: %v", err)
	}
}
