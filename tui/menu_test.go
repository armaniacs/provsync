package main

// コマンドメニューのテスト。子プロセスは引数で応答を変えるスタブで置き換え、
// 呼び出し引数をログファイルに記録して検証する。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// menuStubBin は引数で応答を変えるスタブを作る。"$@" を TUI_TEST_LOG へ追記する。
func menuStubBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "menu-provsync")
	script := `#!/bin/sh
echo "$@" >> "$TUI_TEST_LOG"
case " $* " in
*" --write "*) printf 'wrote: /fake\n';;
*"status --json"*) printf '{"schemaVersion":1,"central":{"path":"/c","exists":true,"providers":0},"tools":[]}';;
*"undo --list"*) printf 'op9  2026-01-01 00:00:00  push x\n    /f\n';;
*"undo"*) printf 'restored: op9\n';;
*"pull "*|*"push "*|*"sync "*|*"init "*) printf 'preview line\n';;
" status ") printf 'central config: /c (0 providers)\n';;
*) printf 'ok output\n';;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func menuTestModel(t *testing.T) (*listModel, string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "args.log")
	t.Setenv("TUI_TEST_LOG", log)
	m := newListModel(menuStubBin(t), testReport(), "en")
	return m, log
}

func downKey() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyDown}
}

func typeText(m *listModel, s string) *listModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return updated.(*listModel)
}

func press(m *listModel, key tea.KeyMsg) (*listModel, tea.Cmd) {
	updated, cmd := m.Update(key)
	return updated.(*listModel), cmd
}

// TestDriftStartupShowsList は drift あり起動で一覧画面から始まることを pin する。
func TestDriftStartupShowsList(t *testing.T) {
	m, _ := menuTestModel(t)
	if m.screen != screenList {
		t.Fatalf("drift startup must open the list, got screen %d", m.screen)
	}
}

// TestListMOpensMenu は一覧の m でメニューが開き、全コマンドが並ぶことを pin する。
func TestListMOpensMenu(t *testing.T) {
	m, _ := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if m2.screen != screenMenu {
		t.Fatalf("m must open the menu, got screen %d", m2.screen)
	}
	view := m2.View()
	for _, name := range []string{"status", "pull", "push", "sync", "diff", "undo", "doctor", "check", "init", "version", "completion", "list"} {
		if !strings.Contains(view, name) {
			t.Errorf("menu view must list %q:\n%s", name, view)
		}
	}
	if !strings.Contains(view, "show the sync state") {
		t.Errorf("menu view must show command descriptions:\n%s", view)
	}
}

// TestMenuEscBackToList はメニューの esc で一覧に戻ることを pin する。
func TestMenuEscBackToList(t *testing.T) {
	m, _ := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3, _ := press(m2, tea.KeyMsg{Type: tea.KeyEsc})
	if m3.screen != screenList {
		t.Errorf("esc must return to the list, got screen %d", m3.screen)
	}
}

// TestMenuStatusRunsAndReturns は引数なしコマンドの実行と復帰を pin する。
func TestMenuStatusRunsAndReturns(t *testing.T) {
	m, log := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuResult {
		t.Fatalf("status must run straight to the result, got screen %d", m3.screen)
	}
	if !strings.Contains(strings.Join(m3.menuOutput, "\n"), "central config:") {
		t.Errorf("result must show the command output: %v", m3.menuOutput)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 1 || calls[0] != "status" {
		t.Errorf("status must run once with no args: %v", calls)
	}
	m4, _ := press(m3, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m4.screen != screenMenu {
		t.Errorf("q must return to the menu, got screen %d", m4.screen)
	}
}

// TestMenuPullPreviewAndApply は pull の入力→プレビュー→適用の全路を pin する。
func TestMenuPullPreviewAndApply(t *testing.T) {
	m, log := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	// pull は 3 項目目(status, list, pull)。
	m3, _ := press(m2, downKey())
	m4, _ := press(m3, downKey())
	updated, _ := m4.Update(enterKey())
	m5 := updated.(*listModel)
	if m5.screen != screenMenuPrompt {
		t.Fatalf("pull must open the prompt, got screen %d", m5.screen)
	}
	if !strings.Contains(m5.View(), "tool:") {
		t.Errorf("prompt must ask for the tool:\n%s", m5.View())
	}
	m6 := typeText(m5, "opencode")
	updated, _ = m6.Update(enterKey())
	m7 := updated.(*listModel)
	if m7.screen != screenMenuPreview {
		t.Fatalf("pull must show the preview, got screen %d", m7.screen)
	}
	if !strings.Contains(strings.Join(m7.menuPreview, "\n"), "preview line") {
		t.Errorf("preview must show the push preview output: %v", m7.menuPreview)
	}
	for _, line := range loggedArgs(t, log) {
		if strings.Contains(line, "--write") {
			t.Fatalf("preview must not use --write: %v", loggedArgs(t, log))
		}
	}
	updated, _ = m7.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m8 := updated.(*listModel)
	if m8.screen != screenMenuResult {
		t.Fatalf("y must run and show the result, got screen %d", m8.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 2 || calls[0] != "pull opencode" || calls[1] != "pull opencode --write" {
		t.Errorf("pull must preview then apply: %v", calls)
	}
}

// TestMenuRequiredPromptRejectsEmpty は必須入力の空送信が無視されることを pin する。
func TestMenuRequiredPromptRejectsEmpty(t *testing.T) {
	m, log := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3, _ := press(m2, downKey())
	m4, _ := press(m3, downKey())
	updated, _ := m4.Update(enterKey())
	m5 := updated.(*listModel)
	updated, _ = m5.Update(enterKey())
	m6 := updated.(*listModel)
	if m6.screen != screenMenuPrompt {
		t.Errorf("empty required input must stay on the prompt, got screen %d", m6.screen)
	}
	if calls := loggedArgs(t, log); len(calls) != 0 {
		t.Errorf("empty submit must run nothing: %v", calls)
	}
}

// TestMenuUndoShowsHistoryAndRuns は undo の履歴表示→確認→実行を pin する。
func TestMenuUndoShowsHistoryAndRuns(t *testing.T) {
	m, log := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3 := m2
	for i := 0; i < 6; i++ {
		m3, _ = press(m3, downKey())
	}
	updated, _ := m3.Update(enterKey())
	m4 := updated.(*listModel)
	if m4.screen != screenMenuPrompt {
		t.Fatalf("undo must open the prompt, got screen %d", m4.screen)
	}
	if !strings.Contains(m4.View(), "op9") {
		t.Errorf("undo prompt must show the history as context:\n%s", m4.View())
	}
	m5 := typeText(m4, "")
	updated, _ = m5.Update(enterKey())
	m6 := updated.(*listModel)
	if m6.screen != screenMenuPreview {
		t.Fatalf("undo must show the pending command, got screen %d", m6.screen)
	}
	if !strings.Contains(strings.Join(m6.menuPreview, "\n"), "provsync undo") {
		t.Errorf("undo preview must show the command: %v", m6.menuPreview)
	}
	updated, _ = m6.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m7 := updated.(*listModel)
	if m7.screen != screenMenuResult {
		t.Fatalf("y must run undo, got screen %d", m7.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 2 || calls[0] != "undo --list" || calls[1] != "undo" {
		t.Errorf("undo must list history then run bare undo: %v", calls)
	}
}

// TestMenuSyncBuildsFromTo は sync の 2 段階入力と引数組み立てを pin する。
func TestMenuSyncBuildsFromTo(t *testing.T) {
	m, log := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	// sync は 5 項目目。
	m3, _ := press(m2, downKey())
	m4, _ := press(m3, downKey())
	m5, _ := press(m4, downKey())
	m6, _ := press(m5, downKey())
	updated, _ := m6.Update(enterKey())
	m7 := updated.(*listModel)
	if !strings.Contains(m7.View(), "source") {
		t.Fatalf("sync must ask for source first:\n%s", m7.View())
	}
	// from を空で確定(任意項目)→ to の入力へ。
	m8 := typeText(m7, "")
	updated, _ = m8.Update(enterKey())
	m9 := updated.(*listModel)
	if !strings.Contains(m9.View(), "target") {
		t.Fatalf("sync must ask for target second:\n%s", m9.View())
	}
	m10 := typeText(m9, "opencode")
	updated, _ = m10.Update(enterKey())
	m11 := updated.(*listModel)
	if m11.screen != screenMenuPreview {
		t.Fatalf("sync must show the preview, got screen %d", m11.screen)
	}
	updated, _ = m11.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m12 := updated.(*listModel)
	if m12.screen != screenMenuResult {
		t.Fatalf("y must run and show the result, got screen %d", m12.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 2 || calls[0] != "sync --to opencode" || calls[1] != "sync --to opencode --write" {
		t.Errorf("sync with empty source must omit --from: %v", calls)
	}
}

// TestMenuPromptEscCancels は入力中の esc が適用せずメニューに戻ることを pin する。
func TestMenuPromptEscCancels(t *testing.T) {
	m, log := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3, _ := press(m2, downKey())
	m4, _ := press(m3, downKey())
	updated, _ := m4.Update(enterKey())
	m5 := updated.(*listModel)
	m6 := typeText(m5, "opencode")
	updated, _ = m6.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m7 := updated.(*listModel)
	if m7.screen != screenMenu {
		t.Errorf("esc must cancel back to the menu, got screen %d", m7.screen)
	}
	if calls := loggedArgs(t, log); len(calls) != 0 {
		t.Errorf("cancel must run nothing: %v", calls)
	}
}

// TestFooterOnListAndMenu は一覧とメニューにフッターが出ることを pin する。
func TestFooterOnListAndMenu(t *testing.T) {
	m, _ := menuTestModel(t)
	if !strings.Contains(m.View(), "? help") {
		t.Errorf("list view must show the footer:\n%s", m.View())
	}
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !strings.Contains(m2.View(), "enter run") {
		t.Errorf("menu view must show the menu footer:\n%s", m2.View())
	}
}

// TestHelpOverlay は ? でヘルプが開き esc/? で閉じることを pin する。
func TestHelpOverlay(t *testing.T) {
	m, _ := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m2.showHelp {
		t.Fatal("? must open the help overlay")
	}
	if !strings.Contains(m2.View(), "shortcuts") {
		t.Errorf("help view must show the title:\n%s", m2.View())
	}
	// ヘルプ表示中は一覧操作が効かない。
	m3, _ := press(m2, downKey())
	if m3.cursor != 0 {
		t.Error("cursor must not move while help is open")
	}
	m4, _ := press(m3, tea.KeyMsg{Type: tea.KeyEsc})
	if m4.showHelp {
		t.Error("esc must close the help overlay")
	}
	m5, _ := press(m4, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m6, _ := press(m5, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if m6.showHelp {
		t.Error("? must toggle the help overlay closed")
	}
}

// TestGTopBottomJump は g/G で先頭・末尾へ飛ぶことを pin する。
func TestGTopBottomJump(t *testing.T) {
	m, _ := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3, _ := press(m2, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if m3.menuCursor != len(menuCommands)-1 {
		t.Errorf("G must jump to the last item, got %d", m3.menuCursor)
	}
	m4, _ := press(m3, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if m4.menuCursor != 0 {
		t.Errorf("g must jump to the first item, got %d", m4.menuCursor)
	}
}

// TestPreviewErrorBlocksApply はプレビュー失敗時に y/enter が無視され、
// 戻りガイドが出ることを pin する(スクリーンショットの混乱の再発防止)。
func TestPreviewErrorBlocksApply(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fail-provsync")
	// status --json には空レポート、push プレビューには失敗を返す。
	report := `{"schemaVersion":1,"central":{"path":"/c","exists":true,"providers":0},"tools":[]}`
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$TUI_TEST_LOG\"\n" +
		"case \" $* \" in\n" +
		"*\"status --json\"*) printf '" + report + "'; exit 1;;\n" +
		"*) printf 'provsync: no supported tool config was found' >&2; exit 1;;\n" +
		"esac\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	log := stubLog(t)
	t.Setenv("TUI_TEST_LOG", log)
	rep := statusReport{SchemaVersion: 1}
	rep.Tools = []toolStatus{{Name: "kilocode", Path: "/k", Exists: true,
		DriftEntries: []driftEntry{{Provider: "llm-01", Op: "drift"}}}}
	m := newListModel(bin, rep, "en")
	// メニュー経由で pull を選び、失敗プレビューまで進める。
	var updated tea.Model
	m4, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m5, _ := press(m4, downKey())
	m6, _ := press(m5, downKey())
	updated, _ = m6.Update(enterKey())
	m7 := updated.(*listModel)
	m8 := typeText(m7, "kilocode")
	updated, _ = m8.Update(enterKey())
	m9 := updated.(*listModel)
	if m9.screen != screenMenuPreview {
		t.Fatalf("must reach the preview, got screen %d", m9.screen)
	}
	if !m9.menuPreviewErr {
		t.Fatal("failed preview must set the error flag")
	}
	if !strings.Contains(m9.View(), "back to menu") {
		t.Errorf("error preview must guide back:\n%s", m9.View())
	}
	before := len(loggedArgs(t, log))
	updated, _ = m9.Update(enterKey())
	m10 := updated.(*listModel)
	if m10.screen != screenMenuPreview {
		t.Errorf("enter on error preview must stay, got screen %d", m10.screen)
	}
	updated, _ = m10.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m11 := updated.(*listModel)
	if m11.screen != screenMenuPreview {
		t.Errorf("y on error preview must stay, got screen %d", m11.screen)
	}
	if got := len(loggedArgs(t, log)); got != before {
		t.Errorf("blocked apply must run nothing: %d calls before and after", before)
	}
	m12, _ := press(m11, tea.KeyMsg{Type: tea.KeyEsc})
	if m12.screen != screenMenu {
		t.Errorf("esc must return to the menu, got screen %d", m12.screen)
	}
}
