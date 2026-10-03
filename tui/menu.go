package main

import (
	"strings"

	"github.com/armaniacs/provsync/internal/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

// menuPromptDef はメニュー項目が求める 1 入力項目の定義。
// labelKey はカタログ ID のリテラルで書く(キー付きリテラル必須。
// internal/i18n の scanCatalogIDRefs が検出できる形式にする)。
type menuPromptDef struct {
	labelKey string
	optional bool
}

// menuCmd はコマンドメニューの 1 項目。CLI のサブコマンドと 1 対 1 に対応する。
// summaryKey は CLI と同じ説明文カタログ(usage.summary.<cmd>)のリテラルで書く。
type menuCmd struct {
	name string
	// prompts は実行前に求める入力。空なら即時実行する。
	prompts []menuPromptDef
	// mutating はファイル書き込みを伴うか。真なら確認を挟んで実行する。
	mutating bool
	// previewFirst は適用前に --write なしプレビューを挟むか
	// (pull/push/sync/init。undo は履歴表示を文脈にする)。
	previewFirst bool
	summaryKey   string
}

// menuCommands はメニューの項目表。CLI のコマンド体系と並行して保守する。
var menuCommands = []menuCmd{
	{name: "status", summaryKey: "usage.summary.status"},
	{name: "list", summaryKey: "usage.summary.list"},
	{name: "pull", prompts: []menuPromptDef{{labelKey: "msg.tui.promptTool", optional: false}}, mutating: true, previewFirst: true, summaryKey: "usage.summary.pull"},
	{name: "push", prompts: []menuPromptDef{{labelKey: "msg.tui.promptTool", optional: false}}, mutating: true, previewFirst: true, summaryKey: "usage.summary.push"},
	{name: "sync", prompts: []menuPromptDef{{labelKey: "msg.tui.promptFrom", optional: true}, {labelKey: "msg.tui.promptTo", optional: false}}, mutating: true, previewFirst: true, summaryKey: "usage.summary.sync"},
	{name: "diff", prompts: []menuPromptDef{{labelKey: "msg.tui.promptFrom", optional: false}, {labelKey: "msg.tui.promptTo", optional: false}}, summaryKey: "usage.summary.diff"},
	{name: "undo", prompts: []menuPromptDef{{labelKey: "msg.tui.promptId", optional: true}}, mutating: true, summaryKey: "usage.summary.undo"},
	{name: "doctor", summaryKey: "usage.summary.doctor"},
	{name: "check", prompts: []menuPromptDef{{labelKey: "msg.tui.promptProvider", optional: true}}, summaryKey: "usage.summary.check"},
	{name: "init", prompts: []menuPromptDef{{labelKey: "msg.tui.promptTool", optional: true}}, mutating: true, previewFirst: true, summaryKey: "usage.summary.init"},
	{name: "version", summaryKey: "usage.summary.version"},
	{name: "completion", prompts: []menuPromptDef{{labelKey: "msg.tui.promptShell", optional: false}}, summaryKey: "usage.summary.completion"},
}

// previewArgs はプレビュー用の引数列を作る。--write は付けない(読み取り専用)。
// undo は履歴表示を文脈として返す。
func (c menuCmd) previewArgs(v []string) []string {
	switch c.name {
	case "sync":
		if v[0] == "" {
			return []string{"sync", "--to", v[1]}
		}
		return []string{"sync", "--from", v[0], "--to", v[1]}
	case "undo":
		return []string{"undo", "--list"}
	case "init":
		if v[0] == "" {
			return []string{"init"}
		}
		return []string{"init", v[0]}
	case "check":
		if v[0] == "" {
			return []string{"check"}
		}
		return []string{"check", "--provider", v[0]}
	case "diff":
		return []string{"diff", v[0], v[1]}
	case "completion":
		return []string{"completion", v[0]}
	case "pull", "push":
		return []string{c.name, v[0]}
	default: // status, list, doctor, version (引数なし)
		return []string{c.name}
	}
}

// applyArgs は適用用の引数列を作る。pull/push/sync/init に --write を付け、
// undo は引数なし(id 指定時はその id)で実行する。
func (c menuCmd) applyArgs(v []string) []string {
	switch c.name {
	case "undo":
		if v[0] == "" {
			return []string{"undo"}
		}
		return []string{"undo", v[0]}
	default:
		return append(c.previewArgs(v), "--write")
	}
}

// updateMenu はコマンドメニューのキー操作を処理する。
func (m *listModel) updateMenu(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if len(m.sel.Items) > 0 {
			m.screen = screenList
			return m, nil
		}
		return m, tea.Quit
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(menuCommands)-1 {
			m.menuCursor++
		}
	case "enter":
		m.activateMenuCmd()
	case "g":
		m.menuCursor = 0
	case "G":
		m.menuCursor = len(menuCommands) - 1
	}
	return m, nil
}

// activateMenuCmd はカーソル位置のコマンドを開始する。入力が要らなければ
// その場で実行し、要ればプロンプト画面へ進む(undo は履歴を文脈に付ける)。
func (m *listModel) activateMenuCmd() {
	c := menuCommands[m.menuCursor]
	m.menuActive = &c
	m.menuValues = nil
	m.menuPrompt = 0
	m.menuInput = nil
	m.menuContext = nil
	if c.name == "init" && m.activateMenuInit() {
		return
	}
	if len(c.prompts) == 0 {
		m.menuOutput, _ = m.runMenuCommand(c.previewArgs(nil))
		m.menuLastCmd = c.name
		m.screen = screenMenuResult
		return
	}
	if c.name == "undo" {
		// 履歴の取得失敗は入力画面に空の文脈として出す(致命的ではない)。
		m.menuContext, _ = m.runMenuCommand([]string{"undo", "--list"})
	}
	m.screen = screenMenuPrompt
}

// activateMenuInit は init 開始時の既存ツール数に応じた分岐を行う。
// 空送信では複数検出時に何も起きない no-op になるため避ける:
// 1 つだけなら指定を省略して進み、存在しなければ空指定で進む
// (最小例つきエラーが表示される)。複数なら必須入力させる。
// 戻り値はプロンプトを出さずに進んだかどうか。
func (m *listModel) activateMenuInit() bool {
	var existing []string
	for _, ts := range m.report.Tools {
		if ts.Exists {
			existing = append(existing, ts.Name)
		}
	}
	switch len(existing) {
	case 0, 1:
		if len(existing) == 1 {
			m.menuValues = []string{existing[0]}
		} else {
			m.menuValues = []string{""}
		}
		m.finishMenuPrompt()
		return true
	default:
		m.menuActive.prompts = []menuPromptDef{{labelKey: "msg.tui.promptTool", optional: false}}
		return false
	}
}

// updateMenuPrompt は引数入力のキー操作を処理する。Enter で確定(必須項目の
// 空送信は無視)、esc でメニューに戻る。カーソル移動のない末尾編集のみ。
func (m *listModel) updateMenuPrompt(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.menuActive
	switch key.Type {
	case tea.KeyEsc:
		m.menuActive = nil
		m.screen = screenMenu
		return m, nil
	case tea.KeyEnter:
		if len(m.menuInput) == 0 && !c.prompts[m.menuPrompt].optional {
			return m, nil
		}
		m.menuValues = append(m.menuValues, string(m.menuInput))
		m.menuInput = nil
		m.menuPrompt++
		if m.menuPrompt < len(c.prompts) {
			return m, nil
		}
		m.finishMenuPrompt()
		return m, nil
	case tea.KeyBackspace:
		if len(m.menuInput) > 0 {
			m.menuInput = m.menuInput[:len(m.menuInput)-1]
		}
		return m, nil
	case tea.KeyRunes:
		m.menuInput = append(m.menuInput, key.Runes...)
		return m, nil
	}
	return m, nil
}

// finishMenuPrompt は全入力の確定後にプレビューまたは実行へ進む。
func (m *listModel) finishMenuPrompt() {
	c := *m.menuActive
	if !c.mutating {
		m.menuOutput, _ = m.runMenuCommand(c.previewArgs(m.menuValues))
		m.menuLastCmd = c.name
		m.screen = screenMenuResult
		return
	}
	if c.previewFirst {
		var err error
		m.menuPreview, err = m.runMenuCommand(c.previewArgs(m.menuValues))
		// プレビュー失敗時の適用は同じ失敗を繰り返すだけなので抑止する。
		m.menuPreviewErr = err != nil
	} else {
		// undo は --write 概念が無いため、実行コマンド自体を表示する。
		m.menuPreview = []string{"provsync " + strings.Join(c.applyArgs(m.menuValues), " ")}
		m.menuPreviewErr = false
	}
	m.screen = screenMenuPreview
}

// updateMenuPreview は適用確認のキー操作を処理する。プレビュー失敗時は
// y/enter を無視し、戻る操作だけを受け付ける。
func (m *listModel) updateMenuPreview(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "y", "enter":
		if m.menuPreviewErr {
			return m, nil
		}
		c := *m.menuActive
		m.menuOutput, _ = m.runMenuCommand(c.applyArgs(m.menuValues))
		m.menuLastCmd = c.name
		m.screen = screenMenuResult
		return m, nil
	case "n", "esc", "q", "ctrl+c":
		m.screen = screenMenu
		return m, nil
	}
	return m, nil
}

// updateMenuResult は結果表示からの復帰を処理する。状態が変わりうるため
// status を取り直してからメニューに戻る(失敗時は古い表示のまま)。
func (m *listModel) updateMenuResult(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc", "enter":
		if rep, err := fetchStatus(m.bin); err == nil {
			m.report = rep
			m.sel = newSelection(rep)
			m.cursor = 0
		}
		m.menuActive = nil
		m.screen = screenMenu
		return m, nil
	case "i":
		if m.showCreateHint() {
			m.activateMenuInitCommand()
			return m, nil
		}
	}
	return m, nil
}

// showCreateHint は status/list の結果画面でセントラル設定が未作成のときに
// その場作成の案内を出すかどうか。m.report は結果画面へ戻るたびに取り直すため、
// 直近の実行結果を反映している。
func (m *listModel) showCreateHint() bool {
	if m.screen != screenMenuResult {
		return false
	}
	if m.menuLastCmd != "status" && m.menuLastCmd != "list" {
		return false
	}
	return !m.report.Central.Exists
}

// activateMenuInitCommand はメニューの init 項目を直接開始する。
// 結果画面からの i ショートカット用に、カーソル位置によらず init を起動する。
func (m *listModel) activateMenuInitCommand() {
	for i, c := range menuCommands {
		if c.name == "init" {
			m.menuCursor = i
			break
		}
	}
	m.activateMenuCmd()
}

// runMenuCommand は子プロセスを同期実行し、出力行と成否を返す。
// 失敗時は stderr を行列に含める。メニューの各操作は単発のローカル実行のため、
// Update 内の同期呼び出しで足りる。
func (m *listModel) runMenuCommand(args []string) ([]string, error) {
	stdout, stderr, err := runProvsync(m.bin, args...)
	var out []string
	if stdout != "" {
		out = append(out, splitLines(stdout)...)
	}
	if err != nil && stderr != "" {
		out = append(out, splitLines(stderr)...)
	}
	return out, err
}

// menuView はコマンドメニューを描画する。説明文は CLI と同じカタログ
// (usage.summary.<cmd>)を流用し、二重管理しない。
func (m *listModel) menuView() string {
	var b strings.Builder
	b.WriteString(i18n.T(m.lang, "msg.tui.menuTitle") + "\n\n")
	for i, c := range menuCommands {
		cursor := "  "
		if i == m.menuCursor {
			cursor = "> "
		}
		b.WriteString(cursor + c.name + " — " + i18n.T(m.lang, c.summaryKey) + "\n")
	}
	return b.String()
}

// menuPromptView は引数入力画面を描画する。
func (m *listModel) menuPromptView() string {
	var b strings.Builder
	for _, line := range m.menuContext {
		b.WriteString(line + "\n")
	}
	if len(m.menuContext) > 0 {
		b.WriteString("\n")
	}
	c := m.menuActive
	b.WriteString(i18n.T(m.lang, c.prompts[m.menuPrompt].labelKey) + string(m.menuInput) + "\n")
	return b.String()
}
