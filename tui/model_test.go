package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testReport() statusReport {
	return statusReport{
		SchemaVersion: 1,
		Central:       centralInfo{Path: "/tmp/config.json", Exists: true, Providers: 3},
		Tools: []toolStatus{
			{Name: "kilocode", Path: "/tmp/kilo.jsonc", Exists: true, Providers: 1,
				Drift: []string{"llm-01: ツールに無い"}},
			{Name: "opencode", Path: "/tmp/opencode.json", Exists: true, Providers: 1,
				Drift: []string{"llm-01: 差分あり", "llm-02: 中央に無い"}},
		},
	}
}

func TestNewSelectionBuildsPairsFromDrift(t *testing.T) {
	s := newSelection(testReport())
	if len(s.Items) != 3 {
		t.Fatalf("items = %d, want 3: %v", len(s.Items), s.Items)
	}
	if s.Items[0] != (pair{Tool: "kilocode", Provider: "llm-01"}) {
		t.Errorf("items[0] = %v", s.Items[0])
	}
}

func TestToggleAndSelectedPairs(t *testing.T) {
	s := newSelection(testReport())
	if len(s.SelectedPairs()) != 0 {
		t.Fatal("nothing selected initially")
	}
	s.Toggle(0)
	s.Toggle(2)
	got := s.SelectedPairs()
	if len(got) != 2 || got[0] != (pair{Tool: "kilocode", Provider: "llm-01"}) || got[1] != (pair{Tool: "opencode", Provider: "llm-02"}) {
		t.Errorf("selected = %v", got)
	}
	s.Toggle(0)
	if len(s.SelectedPairs()) != 1 {
		t.Errorf("toggle must deselect, got %v", s.SelectedPairs())
	}
}

func TestToggleOutOfRangeIsNoop(t *testing.T) {
	s := newSelection(testReport())
	s.Toggle(-1)
	s.Toggle(100)
	if len(s.SelectedPairs()) != 0 {
		t.Errorf("out-of-range toggle must be a noop: %v", s.SelectedPairs())
	}
}

func TestApplyArgsBuildsPushCommands(t *testing.T) {
	s := newSelection(testReport())
	s.Toggle(1)
	args := s.applyArgs()
	if len(args) != 1 {
		t.Fatalf("args = %v", args)
	}
	want := []string{"push", "opencode", "--provider", "llm-01", "--write"}
	for i, w := range want {
		if args[0][i] != w {
			t.Errorf("args[0][%d] = %q, want %q", i, args[0][i], w)
		}
	}
}

func TestCancelWritesNothing(t *testing.T) {
	// 確認画面で n を押すとリストに戻り、子プロセスは実行されない。
	m := newListModel("provsync-unused", testReport())
	m.sel.Toggle(0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*listModel)
	if m2.screen != screenConfirm {
		t.Fatal("enter must move to the confirm screen")
	}
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m3 := updated.(*listModel)
	if m3.screen != screenList {
		t.Errorf("cancel must return to the list screen, got %d", m3.screen)
	}
	if m3.message != nil {
		t.Errorf("cancel must not run any command: %v", m3.message)
	}
}

func TestNonInteractiveStdinIsRejected(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "regular.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isInteractive(f) {
		t.Error("a regular file must not be treated as interactive")
	}
}

func TestViewShowsCommandsWithWrite(t *testing.T) {
	m := newListModel("provsync-unused", testReport())
	m.sel.Toggle(0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*listModel)
	view := m2.View()
	if !strings.Contains(view, "provsync push kilocode --provider llm-01 --write") {
		t.Errorf("confirm view must show the exact command:\n%s", view)
	}
}
