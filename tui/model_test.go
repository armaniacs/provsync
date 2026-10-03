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

func TestNewSelectionPrefersDriftEntries(t *testing.T) {
	r := testReport()
	r.Tools = []toolStatus{
		{Name: "opencode", Path: "/tmp/opencode.json", Exists: true, Providers: 2,
			Drift:        []string{"LLM 01: ツールに無い"},
			DriftEntries: []driftEntry{{Provider: "llm-01", Op: "not-in-tool"}, {Provider: "llm-02", Op: "drift"}}},
	}
	s := newSelection(r)
	if len(s.Items) != 2 {
		t.Fatalf("items = %d, want 2: %v", len(s.Items), s.Items)
	}
	// driftEntries が優先され、drift 文字列の逆解析（"LLM 01"）は使われない
	if s.Items[0] != (pair{Tool: "opencode", Provider: "llm-01"}) {
		t.Errorf("items[0] = %v, want llm-01 from driftEntries", s.Items[0])
	}
	if s.Items[1] != (pair{Tool: "opencode", Provider: "llm-02"}) {
		t.Errorf("items[1] = %v, want llm-02 from driftEntries", s.Items[1])
	}
}

func TestNewSelectionFallsBackToDriftLines(t *testing.T) {
	// driftEntries を含まない旧 CLI の応答では逆解析にフォールバックする
	r := testReport()
	for i := range r.Tools {
		r.Tools[i].DriftEntries = nil
	}
	s := newSelection(r)
	if len(s.Items) != 3 {
		t.Fatalf("items = %d, want 3: %v", len(s.Items), s.Items)
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
	m := newListModel("provsync-unused", testReport(), "ja")
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

// TestApplyDoneMsgMovesToDoneAndQuit は list→confirm→(applyDoneMsg)→done→q の
// 一連の遷移を Update 呼び出しで pin する。done 遷移は Msg 経由のみで起こる。
func TestApplyDoneMsgMovesToDoneAndQuit(t *testing.T) {
	m := newListModel("provsync-unused", testReport(), "ja")
	m.sel.Toggle(0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*listModel)
	if m2.screen != screenConfirm {
		t.Fatal("enter must move to the confirm screen")
	}
	updated, cmd := m2.Update(applyDoneMsg{messages: []string{"applied: provsync push", "ok"}})
	m3 := updated.(*listModel)
	if m3.screen != screenDone {
		t.Errorf("applyDoneMsg must move to the done screen, got %d", m3.screen)
	}
	want := []string{"applied: provsync push", "ok"}
	if len(m3.message) != len(want) {
		t.Fatalf("message = %v, want %v", m3.message, want)
	}
	for i := range want {
		if m3.message[i] != want[i] {
			t.Errorf("message[%d] = %q, want %q", i, m3.message[i], want[i])
		}
	}
	if cmd == nil {
		t.Fatal("applyDoneMsg must request quit")
	}
	updated, cmd = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("q on the done screen must quit")
	}
}

// TestApplySelectedCmdReturnsMsgWithoutMutatingModel は Cmd が別 goroutine で動く
// ため、クロージャからモデルを読み書きしてはいけない(bubbletea の契約)ことを pin する。
func TestApplySelectedCmdReturnsMsgWithoutMutatingModel(t *testing.T) {
	bin := fakeBin(t, "pushed ok")
	m := newListModel(bin, testReport(), "ja")
	m.sel.Toggle(0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*listModel)
	cmd := m2.applySelected()
	if cmd == nil {
		t.Fatal("applySelected must return a cmd")
	}
	msg := cmd()
	done, ok := msg.(applyDoneMsg)
	if !ok {
		t.Fatalf("cmd must return applyDoneMsg, got %T: %v", msg, msg)
	}
	if len(done.messages) != 1 {
		t.Fatalf("messages = %v, want 1 entry", done.messages)
	}
	if !strings.Contains(done.messages[0], "provsync push kilocode --provider llm-01 --write") ||
		!strings.Contains(done.messages[0], "pushed ok") {
		t.Errorf("message must contain the joined command and stdout: %q", done.messages[0])
	}
	if m2.screen != screenConfirm || m2.message != nil {
		t.Errorf("cmd must not mutate the model: screen=%d, message=%v", m2.screen, m2.message)
	}
}

// TestApplySelectedCmdReportsFailure は子プロセス失敗時に failed 文言の
// メッセージ列が applyDoneMsg で返ることを pin する。
func TestApplySelectedCmdReportsFailure(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fail-provsync")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'boom' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := newListModel(bin, testReport(), "en")
	m.sel.Toggle(0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*listModel)
	done, ok := m2.applySelected()().(applyDoneMsg)
	if !ok {
		t.Fatal("cmd must return applyDoneMsg")
	}
	if len(done.messages) != 1 {
		t.Fatalf("messages = %v, want 1 entry", done.messages)
	}
	if !strings.Contains(done.messages[0], "failed: provsync push kilocode --provider llm-01 --write") ||
		!strings.Contains(done.messages[0], "boom") {
		t.Errorf("failure message must contain the command and stderr: %q", done.messages[0])
	}
}

// TestConfirmEscReturnsToList は確認画面で esc を押すとリストに戻ることを pin する
// (n の復帰は TestCancelWritesNothing が担当)。
func TestConfirmEscReturnsToList(t *testing.T) {
	m := newListModel("provsync-unused", testReport(), "ja")
	m.sel.Toggle(0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*listModel)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated.(*listModel)
	if m3.screen != screenList {
		t.Errorf("esc must return to the list screen, got %d", m3.screen)
	}
	if m3.message != nil {
		t.Errorf("esc must not run any command: %v", m3.message)
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
	m := newListModel("provsync-unused", testReport(), "ja")
	m.sel.Toggle(0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*listModel)
	view := m2.View()
	if !strings.Contains(view, "provsync push kilocode --provider llm-01 --write") {
		t.Errorf("confirm view must show the exact command:\n%s", view)
	}
}
