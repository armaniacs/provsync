package diff

import (
	"strings"
	"testing"
)

func TestUnifiedEqualReturnsEmpty(t *testing.T) {
	if got := Unified("f", []byte("a\nb\n"), []byte("a\nb\n"), 3); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestUnifiedReplacement(t *testing.T) {
	before := "a\nb\nc\n"
	after := "a\nB\nc\n"
	got := Unified("opencode.json", []byte(before), []byte(after), 3)
	for _, want := range []string{"--- a/opencode.json", "+++ b/opencode.json", "@@", "-b", "+B", " a", " c"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff missing %q:\n%s", want, got)
		}
	}
}

func TestUnifiedNewFile(t *testing.T) {
	got := Unified("f", nil, []byte("x\ny\n"), 3)
	if !strings.Contains(got, "-0,0") {
		t.Errorf("new file header wrong:\n%s", got)
	}
	if !strings.Contains(got, "+x") || !strings.Contains(got, "+y") {
		t.Errorf("added lines missing:\n%s", got)
	}
}

func TestUnifiedDeletion(t *testing.T) {
	got := Unified("f", []byte("x\ny\n"), nil, 3)
	if !strings.Contains(got, "-x") || !strings.Contains(got, "-y") {
		t.Errorf("deleted lines missing:\n%s", got)
	}
	if !strings.Contains(got, "+0,0") {
		t.Errorf("new file header wrong:\n%s", got)
	}
}

func TestUnifiedExact(t *testing.T) {
	got := Unified("f", []byte("a\nb\nc\n"), []byte("a\nB\nc\n"), 3)
	want := "--- a/f\n+++ b/f\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestUnifiedAbsolutePathHeader(t *testing.T) {
	got := Unified("/home/you/config.json", []byte("a\n"), []byte("b\n"), 3)
	if !strings.Contains(got, "--- a/home/you/config.json\n") {
		t.Errorf("absolute path header should not duplicate slash:\n%s", got)
	}
	if strings.Contains(got, "a//") {
		t.Errorf("duplicated slash in header:\n%s", got)
	}
}

func TestUnifiedMultiHunk(t *testing.T) {
	var before, after strings.Builder
	for i := 1; i <= 30; i++ {
		before.WriteString("line\n")
		if i == 3 || i == 27 {
			after.WriteString("changed\n")
		} else {
			after.WriteString("line\n")
		}
	}
	got := Unified("f", []byte(before.String()), []byte(after.String()), 3)
	if n := strings.Count(got, "@@ -"); n != 2 {
		t.Errorf("expected 2 hunks, got %d:\n%s", n, got)
	}
}
