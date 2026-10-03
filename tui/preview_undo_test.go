package main

// 確認画面の差分プレビューと done 画面の undo のテスト。
// 子プロセスは引数で応答を変えるシェルスタブで置き換え、呼び出し引数を
// ログファイルに記録して検証する。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// argStubBin は引数で応答を変えるスタブを作る。"$@" を logPath へ追記し、
// --write 付き push には適用出力を、--write なし push にはプレビュー出力を、
// undo には復元出力を返す。それ以外は失敗する。
func argStubBin(t *testing.T, logPath string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "arg-provsync")
	script := `#!/bin/sh
echo "$@" >> "$TUI_TEST_LOG"
case " $* " in
*" --write "*) printf 'opencode: /fake\n  llm-01: added\nbackup: 0\nwrote: /fake\n';;
*"push "*) printf 'opencode: /fake\n  llm-01: added\n(preview only; apply with --write)\n';;
*"undo"*) printf 'restored: 0 (push opencode)\n  restored: /fake\n';;
*) printf 'unexpected: %s\n' "$*"; exit 1;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func stubLog(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "args.log")
}

func loggedArgs(t *testing.T, logPath string) []string {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if err != nil {
		// 子プロセス未実行ではログが作られない。呼び出しゼロとして扱う。
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func enterKey() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

// TestPreviewArgsOmitWriteFlag はプレビュー引数に --write が含まれないこと
// (読み取り専用であること)を pin する。
func TestPreviewArgsOmitWriteFlag(t *testing.T) {
	m := newListModel("provsync-unused", testReport(), "en")
	m.sel.Toggle(0)
	argsList := m.sel.previewArgs()
	if len(argsList) != 1 {
		t.Fatalf("previewArgs = %v, want 1 entry", argsList)
	}
	args := argsList[0]
	if len(args) != 4 || args[0] != "push" || args[1] != "kilocode" || args[2] != "--provider" || args[3] != "llm-01" {
		t.Errorf("previewArgs = %q, want [push kilocode --provider llm-01]", args)
	}
}

// TestEnterFetchesPreviewsAndShowsThem は確認画面へ進むとプレビュー取得 Cmd が
// 返り、その完了で View に変更内容が出ることを pin する。
func TestEnterFetchesPreviewsAndShowsThem(t *testing.T) {
	log := stubLog(t)
	t.Setenv("TUI_TEST_LOG", log)
	m := newListModel(argStubBin(t, log), testReport(), "en")
	m.sel.Toggle(0)
	updated, cmd := m.Update(enterKey())
	m2 := updated.(*listModel)
	if m2.screen != screenConfirm {
		t.Fatal("enter must move to the confirm screen")
	}
	if cmd == nil {
		t.Fatal("enter must return a preview fetch cmd")
	}
	msg := cmd()
	done, ok := msg.(previewDoneMsg)
	if !ok {
		t.Fatalf("preview cmd must return previewDoneMsg, got %T", msg)
	}
	updated, _ = m2.Update(done)
	m3 := updated.(*listModel)
	if len(m3.previews) != 1 {
		t.Fatalf("previews = %v, want 1 entry", m3.previews)
	}
	view := m3.View()
	if !strings.Contains(view, "provsync push kilocode --provider llm-01 --write") {
		t.Errorf("confirm view must show the apply command:\n%s", view)
	}
	if !strings.Contains(view, "llm-01: added") {
		t.Errorf("confirm view must show the preview lines:\n%s", view)
	}
	for _, line := range loggedArgs(t, log) {
		if strings.Contains(line, "--write") {
			t.Errorf("preview must not use --write: %q", line)
		}
	}
}

// TestPreviewFailureShowsError はプレビュー取得の失敗が確認画面に
// エラー表示されることを pin する。
func TestPreviewFailureShowsError(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fail-provsync")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'cannot read' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := newListModel(bin, testReport(), "en")
	m.sel.Toggle(0)
	updated, cmd := m.Update(enterKey())
	m2 := updated.(*listModel)
	done, ok := cmd().(previewDoneMsg)
	if !ok {
		t.Fatal("preview cmd must return previewDoneMsg")
	}
	updated, _ = m2.Update(done)
	m3 := updated.(*listModel)
	if !strings.Contains(m3.View(), "failed:") || !strings.Contains(m3.View(), "cannot read") {
		t.Errorf("confirm view must show the preview error:\n%s", m3.View())
	}
}

// TestDoneUndoRevertsLastOperation は done 画面の u → 確認 → y で
// 引数なし undo が実行され、結果が表示されることを pin する。
func TestDoneUndoRevertsLastOperation(t *testing.T) {
	log := stubLog(t)
	t.Setenv("TUI_TEST_LOG", log)
	m := newListModel(argStubBin(t, log), testReport(), "en")
	m.sel.Toggle(0)
	updated, _ := m.Update(enterKey())
	m2 := updated.(*listModel)
	updated, _ = m2.Update(applyDoneMsg{messages: []string{"applied: ok"}})
	m3 := updated.(*listModel)
	if m3.screen != screenDone {
		t.Fatal("applyDoneMsg must move to the done screen")
	}
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m4 := updated.(*listModel)
	if m4.screen != screenConfirmUndo {
		t.Fatalf("u must move to the undo confirm screen, got %d", m4.screen)
	}
	if !strings.Contains(m4.View(), "revert the last operation?") {
		t.Errorf("undo confirm view must ask: %q", m4.View())
	}
	updated, cmd := m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m5 := updated.(*listModel)
	if cmd == nil {
		t.Fatal("y must return an undo cmd")
	}
	_ = m5
	msg := cmd()
	done, ok := msg.(applyDoneMsg)
	if !ok {
		t.Fatalf("undo cmd must return applyDoneMsg, got %T", msg)
	}
	updated, _ = m5.Update(done)
	m6 := updated.(*listModel)
	if m6.screen != screenDone {
		t.Errorf("undo result must return to the done screen, got %d", m6.screen)
	}
	if !strings.Contains(strings.Join(m6.message, "\n"), "restored:") {
		t.Errorf("undo result must show the restore output: %v", m6.message)
	}
	calls := loggedArgs(t, log)
	found := false
	for _, line := range calls {
		if line == "undo" {
			found = true
		}
		if strings.Contains(line, "--write") {
			t.Errorf("undo must not use --write: %q", line)
		}
	}
	if !found {
		t.Errorf("undo must run bare `undo`: calls = %v", calls)
	}
}

// TestConfirmUndoCancelKeepsResults は取り消し確認で n を押すと
// 適用結果を保ったまま done 画面に戻ることを pin する。
func TestConfirmUndoCancelKeepsResults(t *testing.T) {
	log := stubLog(t)
	t.Setenv("TUI_TEST_LOG", log)
	m := newListModel(argStubBin(t, log), testReport(), "en")
	updated, _ := m.Update(applyDoneMsg{messages: []string{"applied: ok"}})
	m2 := updated.(*listModel)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m3 := updated.(*listModel)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m4 := updated.(*listModel)
	if m4.screen != screenDone {
		t.Fatalf("n must return to the done screen, got %d", m4.screen)
	}
	if len(m4.message) != 1 || m4.message[0] != "applied: ok" {
		t.Errorf("cancel must keep the apply results: %v", m4.message)
	}
	if calls := loggedArgs(t, log); len(calls) != 0 {
		t.Errorf("cancel must run no subprocess: %v", calls)
	}
}
