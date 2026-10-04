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
*"cleanup --yes"*) printf 'deleted: /c\n';;
" cleanup ") printf 'the following will be removed:\n  /c (1.0 KB)\n';;
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
// openMenuAt はメニューを開いて idx 項目へ移動する。
func openMenuAt(m *listModel, idx int) *listModel {
	cur, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	for i := 0; i < idx; i++ {
		cur, _ = press(cur, downKey())
	}
	return cur
}

func TestMenuPullPreviewAndApply(t *testing.T) {
	m, log := menuTestModel(t)
	// pull は 3 項目目(status, list, pull)。
	m2 := openMenuAt(m, 2)
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPick {
		t.Fatalf("pull must open the tool picker, got screen %d", m3.screen)
	}
	if !strings.Contains(m3.View(), "kilocode") || !strings.Contains(m3.View(), "opencode") {
		t.Errorf("picker must list existing tools:\n%s", m3.View())
	}
	// opencode(2 行目)を選ぶ。
	m4, _ := press(m3, downKey())
	updated, _ = m4.Update(enterKey())
	m5 := updated.(*listModel)
	if m5.screen != screenMenuPreview {
		t.Fatalf("pull must show the preview, got screen %d", m5.screen)
	}
	if !strings.Contains(strings.Join(m5.menuPreview, "\n"), "preview line") {
		t.Errorf("preview must show the push preview output: %v", m5.menuPreview)
	}
	for _, line := range loggedArgs(t, log) {
		if strings.Contains(line, "--write") {
			t.Fatalf("preview must not use --write: %v", loggedArgs(t, log))
		}
	}
	updated, _ = m5.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m6 := updated.(*listModel)
	if m6.screen != screenMenuResult {
		t.Fatalf("y must run and show the result, got screen %d", m6.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 2 || calls[0] != "pull opencode" || calls[1] != "pull opencode --write" {
		t.Errorf("pull must preview then apply: %v", calls)
	}
}

// TestMenuRequiredPromptRejectsEmpty は必須の自由入力の空送信が
// 無視されることを pin する(completion のシェル名)。
func TestMenuRequiredPromptRejectsEmpty(t *testing.T) {
	m, log := menuTestModel(t)
	// completion は 12 項目目。
	m2 := openMenuAt(m, 11)
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPrompt {
		t.Fatalf("completion must open the prompt, got screen %d", m3.screen)
	}
	updated, _ = m3.Update(enterKey())
	m4 := updated.(*listModel)
	if m4.screen != screenMenuPrompt {
		t.Errorf("empty required input must stay on the prompt, got screen %d", m4.screen)
	}
	if calls := loggedArgs(t, log); len(calls) != 0 {
		t.Errorf("empty submit must run nothing: %v", calls)
	}
}

// TestMenuPickEmpty はツール無しで選択肢が空のとき決定が無視されることを pin する。
func TestMenuPickEmpty(t *testing.T) {
	log := stubLog(t)
	t.Setenv("TUI_TEST_LOG", log)
	r := testReport()
	for i := range r.Tools {
		r.Tools[i].Exists = false
	}
	m := newListModel(menuStubBin(t), r, "en")
	// 選択肢なしでもメニューは開く。
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3, _ := press(m2, downKey())
	m4, _ := press(m3, downKey())
	updated, _ := m4.Update(enterKey())
	m5 := updated.(*listModel)
	if m5.screen != screenMenuPick {
		t.Fatalf("pull must open the picker, got screen %d", m5.screen)
	}
	if !strings.Contains(m5.View(), "no tools") {
		t.Errorf("empty picker must say so:\n%s", m5.View())
	}
	updated, _ = m5.Update(enterKey())
	m6 := updated.(*listModel)
	if m6.screen != screenMenuPick {
		t.Errorf("enter on empty picker must stay, got screen %d", m6.screen)
	}
	if calls := loggedArgs(t, log); len(calls) != 0 {
		t.Errorf("empty pick must run nothing: %v", calls)
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

// TestMenuSyncBuildsFromTo は sync の 2 段階選択と引数組み立てを pin する。
// from の先頭行(セントラル設定)=空、to の opencode 行を選ぶ。
func TestMenuSyncBuildsFromTo(t *testing.T) {
	m, log := menuTestModel(t)
	// sync は 5 項目目。
	m2 := openMenuAt(m, 4)
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPick {
		t.Fatalf("sync must open the picker, got screen %d", m3.screen)
	}
	if !strings.Contains(m3.View(), "source") {
		t.Fatalf("sync must ask for source first:\n%s", m3.View())
	}
	// from の先頭行(セントラル設定=空)を選ぶ→ to の選択へ。
	updated, _ = m3.Update(enterKey())
	m4 := updated.(*listModel)
	if m4.screen != screenMenuPick {
		t.Fatalf("sync must ask for target second, got screen %d", m4.screen)
	}
	if !strings.Contains(m4.View(), "target") {
		t.Fatalf("sync second pick must ask for target:\n%s", m4.View())
	}
	// to の opencode 行(先頭セントラル行の次)を選ぶ。
	m5, _ := press(m4, downKey())
	updated, _ = m5.Update(enterKey())
	m6 := updated.(*listModel)
	if m6.screen != screenMenuPreview {
		t.Fatalf("sync must show the preview, got screen %d", m6.screen)
	}
	updated, _ = m6.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m7 := updated.(*listModel)
	if m7.screen != screenMenuResult {
		t.Fatalf("y must run and show the result, got screen %d", m7.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 2 || calls[0] != "sync --to opencode" || calls[1] != "sync --to opencode --write" {
		t.Errorf("sync with empty source must omit --from: %v", calls)
	}
}

// TestMenuPickEscCancels は選択中の esc が適用せずメニューに戻ることを pin する。
func TestMenuPickEscCancels(t *testing.T) {
	m, log := menuTestModel(t)
	m2 := openMenuAt(m, 2)
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPick {
		t.Fatalf("pull must open the picker, got screen %d", m3.screen)
	}
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := updated.(*listModel)
	if m4.screen != screenMenu {
		t.Errorf("esc must cancel back to the menu, got screen %d", m4.screen)
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

// TestMenuInitSingleToolSkipsPrompt はツール1つのときに init が入力を
// 求めず、そのツールでプレビューへ進むことを pin する。
func TestMenuInitSingleToolSkipsPrompt(t *testing.T) {
	log := stubLog(t)
	t.Setenv("TUI_TEST_LOG", log)
	r := testReport()
	r.Tools = r.Tools[:1]
	m := newListModel(menuStubBin(t), r, "en")
	m0, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	// init は 10 項目目。
	m2 := m0
	for i := 0; i < 9; i++ {
		m2, _ = press(m2, downKey())
	}
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPreview {
		t.Fatalf("single-tool init must skip the prompt, got screen %d", m3.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 1 || calls[0] != "init kilocode" {
		t.Errorf("single-tool init must preview with the tool: %v", calls)
	}
}

// TestMenuInitNoToolsRunsBareInit はツール無しで init が空指定のまま進み、
// 最小例つきエラーのプレビューになることを pin する。
func TestMenuInitNoToolsRunsBareInit(t *testing.T) {
	log := stubLog(t)
	t.Setenv("TUI_TEST_LOG", log)
	r := testReport()
	for i := range r.Tools {
		r.Tools[i].Exists = false
	}
	m := newListModel(menuStubBin(t), r, "en")
	m2 := m
	for i := 0; i < 9; i++ {
		m2, _ = press(m2, downKey())
	}
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPreview {
		t.Fatalf("tool-less init must skip the prompt, got screen %d", m3.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 1 || calls[0] != "init" {
		t.Errorf("tool-less init must preview bare init: %v", calls)
	}
}

// TestMenuInitMultipleToolsRequiresTool は複数ツールで init が選択式になり、
// 選ばずに進めないこと(空送信の no-op を防ぐ)を pin する。
func TestMenuInitMultipleToolsRequiresTool(t *testing.T) {
	m, log := menuTestModel(t)
	// init は 10 項目目。
	m2 := openMenuAt(m, 9)
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPick {
		t.Fatalf("multi-tool init must open the picker, got screen %d", m3.screen)
	}
	if !strings.Contains(m3.View(), "kilocode") || !strings.Contains(m3.View(), "opencode") {
		t.Fatalf("init picker must list tools:\n%s", m3.View())
	}
	updated, _ = m3.Update(enterKey())
	m4 := updated.(*listModel)
	if m4.screen != screenMenuPreview {
		t.Fatalf("init with picked tool must show the preview, got screen %d", m4.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 1 || calls[0] != "init kilocode" {
		t.Errorf("init must preview with the picked tool: %v", calls)
	}
}

// centralMissingReport は中央未作成・ツール2件のレポートを作る。
func centralMissingReport() statusReport {
	return statusReport{
		SchemaVersion: 1,
		Central:       centralInfo{Path: "/c", Exists: false},
		Tools: []toolStatus{
			{Name: "kilocode", Path: "/k", Exists: true, Providers: 1,
				DriftEntries: []driftEntry{{Provider: "llm-01", Op: "drift"}}},
			{Name: "opencode", Path: "/o", Exists: true, Providers: 1,
				DriftEntries: []driftEntry{{Provider: "llm-02", Op: "drift"}}},
		},
	}
}

// TestMenuResultCreateHint は中央未作成の status 結果で作成案内が出て、
// i で init フローに入ることを pin する。
func TestMenuResultCreateHint(t *testing.T) {
	m, log := menuTestModel(t)
	m.report = centralMissingReport()
	m.sel = newSelection(m.report)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if m3.screen != screenMenuResult {
		t.Fatalf("status must run to the result, got screen %d", m3.screen)
	}
	if !strings.Contains(m3.View(), "press i to create it now") {
		t.Errorf("result must guide central creation:\n%s", m3.View())
	}
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m4 := updated.(*listModel)
	// ツール2件のため init は選択式になる。
	if m4.screen != screenMenuPick {
		t.Fatalf("i must start the init flow, got screen %d", m4.screen)
	}
	// kilocode 行(先頭)を選ぶ。
	updated, _ = m4.Update(enterKey())
	m6 := updated.(*listModel)
	if m6.screen != screenMenuPreview {
		t.Fatalf("init with picked tool must show the preview, got screen %d", m6.screen)
	}
	calls := loggedArgs(t, log)
	found := false
	for _, line := range calls {
		if line == "init kilocode" {
			found = true
		}
	}
	if !found {
		t.Errorf("i flow must preview init with the picked tool: %v", calls)
	}
}

// TestMenuResultNoHintWhenCentralExists は中央作成済みで案内が出ず、
// i が無視されることを pin する。
func TestMenuResultNoHintWhenCentralExists(t *testing.T) {
	m, _ := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	updated, _ := m2.Update(enterKey())
	m3 := updated.(*listModel)
	if strings.Contains(m3.View(), "press i to create it now") {
		t.Errorf("result must not guide creation when central exists:\n%s", m3.View())
	}
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m4 := updated.(*listModel)
	if m4.screen != screenMenuResult {
		t.Errorf("i must be ignored without the hint, got screen %d", m4.screen)
	}
}

// TestMenuResultNoHintForOtherCommands は status/list 以外で案内が出ないことを pin する。
func TestMenuResultNoHintForOtherCommands(t *testing.T) {
	m, _ := menuTestModel(t)
	m.report = centralMissingReport()
	m.sel = newSelection(m.report)
	// doctor は 8 項目目。
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m3 := m2
	for i := 0; i < 7; i++ {
		m3, _ = press(m3, downKey())
	}
	updated, _ := m3.Update(enterKey())
	m4 := updated.(*listModel)
	if m4.screen != screenMenuResult {
		t.Fatalf("doctor must run to the result, got screen %d", m4.screen)
	}
	if strings.Contains(m4.View(), "press i to create it now") {
		t.Errorf("non-status result must not guide creation:\n%s", m4.View())
	}
}

func TestMenuShowsCleanup(t *testing.T) {
	m, _ := menuTestModel(t)
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !strings.Contains(m2.View(), "cleanup") {
		t.Errorf("menu must list cleanup:\n%s", m2.View())
	}
}

func TestMenuCleanupPreviewAndApply(t *testing.T) {
	m, log := menuTestModel(t)
	// cleanup is the last menu item: jump with G.
	m2, _ := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	updated, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m3 := updated.(*listModel)
	updated, _ = m3.Update(enterKey())
	m4 := updated.(*listModel)
	if m4.screen != screenMenuPreview {
		t.Fatalf("cleanup must show the preview, got screen %d", m4.screen)
	}
	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m5 := updated.(*listModel)
	if m5.screen != screenMenuResult {
		t.Fatalf("y must run and show the result, got screen %d", m5.screen)
	}
	calls := loggedArgs(t, log)
	if len(calls) != 2 || calls[0] != "cleanup" || calls[1] != "cleanup --yes" {
		t.Errorf("cleanup must preview then run with --yes: %v", calls)
	}
}
