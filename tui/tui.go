package main

import (
	"strings"

	"github.com/armaniacs/provsync/internal/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenList screen = iota
	screenConfirm
	screenDone
	screenConfirmUndo
)

// previewResult は 1 ペア分の push プレビュー(--write なし)の結果。
// lines は子プロセスの標準出力行で、失敗時は errText に標準エラーが入る。
type previewResult struct {
	args    []string
	lines   []string
	errText string
}

// previewDoneMsg はプレビュー取得 Cmd の完了結果。
type previewDoneMsg struct {
	results []previewResult
}

// listModel は bubbletea のモデル。選択状態(selection)を操作する薄い層で、
// 変更内容の計算は provsync 子プロセスに任せる。
type listModel struct {
	bin      string
	lang     string
	report   statusReport
	sel      *selection
	cursor   int
	screen   screen
	message  []string
	previews []previewResult
}

func newListModel(bin string, report statusReport, lang string) *listModel {
	return &listModel{
		bin:    bin,
		lang:   lang,
		report: report,
		sel:    newSelection(report),
		screen: screenList,
	}
}

func (m *listModel) Init() tea.Cmd { return nil }

// applyDoneMsg は適用 Cmd の完了結果。Cmd は別 goroutine で動くため、
// モデル変更はこの Msg を Update で処理して行う(クロージャからは触らない)。
type applyDoneMsg struct {
	messages []string
}

func (m *listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if done, ok := msg.(applyDoneMsg); ok {
		m.message = done.messages
		m.screen = screenDone
		return m, tea.Quit
	}
	if done, ok := msg.(previewDoneMsg); ok {
		m.previews = done.results
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.screen {
	case screenList:
		return m.updateList(key)
	case screenConfirm:
		return m.updateConfirm(key)
	case screenConfirmUndo:
		return m.updateConfirmUndo(key)
	default:
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "u":
			m.screen = screenConfirmUndo
			return m, nil
		}
		return m, nil
	}
}

func (m *listModel) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.sel.Items)-1 {
			m.cursor++
		}
	case " ":
		m.sel.Toggle(m.cursor)
	case "enter":
		if len(m.sel.SelectedPairs()) > 0 {
			m.screen = screenConfirm
			m.previews = nil
			return m, m.fetchPreviews()
		}
	}
	return m, nil
}

func (m *listModel) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "n", "esc", "ctrl+c":
		m.screen = screenList
		return m, nil
	case "y", "enter":
		return m, m.applySelected()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *listModel) updateConfirmUndo(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "y", "enter":
		return m, m.applyUndo()
	case "n", "esc", "q", "ctrl+c":
		m.screen = screenDone
		return m, nil
	}
	return m, nil
}

// fetchPreviews は選択済みペアの push プレビュー(--write なし)を非同期で取得する。
// Cmd は別 goroutine で動くため、モデルは Msg 経由でのみ更新する。
func (m *listModel) fetchPreviews() tea.Cmd {
	bin := m.bin
	argsList := m.sel.previewArgs()
	return func() tea.Msg {
		var out []previewResult
		for _, args := range argsList {
			stdout, stderr, err := runProvsync(bin, args...)
			r := previewResult{args: args}
			if err != nil {
				r.errText = strings.TrimSpace(stderr)
			} else {
				r.lines = splitLines(stdout)
			}
			out = append(out, r)
		}
		return previewDoneMsg{results: out}
	}
}

// applyUndo は直前の操作を取り消す undo(引数なし)を非同期で実行する。
// 結果は applyDoneMsg 経由で done 画面に表示され、u の繰り返しが効く。
func (m *listModel) applyUndo() tea.Cmd {
	bin := m.bin
	lang := m.lang
	return func() tea.Msg {
		stdout, stderr, err := runProvsync(bin, "undo")
		var out []string
		if err != nil {
			out = append(out, i18n.T(lang, "msg.tui.failed", "undo", stderr))
		} else {
			out = append(out, i18n.T(lang, "msg.tui.apply", "undo", stdout))
		}
		return applyDoneMsg{messages: out}
	}
}

// splitLines は出力を空行を除いた行列に分ける。
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// applySelected は承認されたときだけ --write 付きで provsync 子プロセスを実行する。
// 子プロセスの出力は子プロセス自身が env から解決した言語で出る。
// モデルの読み取りは Cmd 外で済ませる: Cmd は別 goroutine で動くため、
// クロージャ内で m に触れるとイベントループと競合する。
func (m *listModel) applySelected() tea.Cmd {
	bin := m.bin
	lang := m.lang
	argsList := m.sel.applyArgs()
	return func() tea.Msg {
		var out []string
		for _, args := range argsList {
			joined := strings.Join(args, " ")
			stdout, stderr, err := runProvsync(bin, args...)
			if err != nil {
				out = append(out, i18n.T(lang, "msg.tui.failed", joined, stderr))
				continue
			}
			out = append(out, i18n.T(lang, "msg.tui.apply", joined, stdout))
		}
		return applyDoneMsg{messages: out}
	}
}

func (m *listModel) View() string {
	var b strings.Builder
	b.WriteString("provsync-tui\n\n")
	b.WriteString(i18n.T(m.lang, "msg.tui.central", m.report.Central.Path) + "\n\n")

	switch m.screen {
	case screenList:
		if len(m.sel.Items) == 0 {
			b.WriteString(i18n.T(m.lang, "msg.tui.noDrift") + "\n\n")
			b.WriteString(i18n.T(m.lang, "msg.tui.quit") + "\n")
			return b.String()
		}
		b.WriteString(i18n.T(m.lang, "msg.tui.listHint") + "\n\n")
		for i, p := range m.sel.Items {
			check := " "
			if m.sel.Selected[p] {
				check = "x"
			}
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			b.WriteString(cursor + "[" + check + "] " + p.Tool + " / " + p.Provider + "\n")
		}
	case screenConfirm:
		b.WriteString(i18n.T(m.lang, "msg.tui.confirmHeader") + "\n\n")
		for i, args := range m.sel.applyArgs() {
			b.WriteString("  provsync " + strings.Join(args, " ") + "\n")
			b.WriteString(m.previewText(i))
		}
		b.WriteString("\n" + i18n.T(m.lang, "msg.tui.confirmPrompt") + "\n")
	case screenConfirmUndo:
		b.WriteString(i18n.T(m.lang, "msg.tui.confirmUndo") + "\n")
	case screenDone:
		b.WriteString(strings.Join(m.message, "\n"))
		b.WriteString("\n\n" + i18n.T(m.lang, "msg.tui.doneHint") + "\n")
	}
	return b.String()
}

// previewText は i 番目の適用コマンドに対応するプレビュー表示を返す。
// 取得前は読み込み中、失敗時はエラーを表示する。
func (m *listModel) previewText(i int) string {
	if i < 0 || i >= len(m.previews) {
		return "    " + i18n.T(m.lang, "msg.tui.previewLoading") + "\n"
	}
	r := m.previews[i]
	if r.errText != "" {
		return "    " + i18n.T(m.lang, "msg.tui.failed", strings.Join(r.args, " "), r.errText) + "\n"
	}
	var b strings.Builder
	for _, line := range r.lines {
		b.WriteString("    " + line + "\n")
	}
	return b.String()
}
