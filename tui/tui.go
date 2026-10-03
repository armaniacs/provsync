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
)

// listModel は bubbletea のモデル。選択状態(selection)を操作する薄い層で、
// 変更内容の計算は provsync 子プロセスに任せる。
type listModel struct {
	bin     string
	lang    string
	report  statusReport
	sel     *selection
	cursor  int
	screen  screen
	message []string
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

func (m *listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.screen {
	case screenList:
		return m.updateList(key)
	case screenConfirm:
		return m.updateConfirm(key)
	default:
		if key.String() == "q" || key.String() == "ctrl+c" {
			return m, tea.Quit
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

// applySelected は承認されたときだけ --write 付きで provsync 子プロセスを実行する。
// 子プロセスの出力は子プロセス自身が env から解決した言語で出る。
func (m *listModel) applySelected() tea.Cmd {
	return func() tea.Msg {
		var out []string
		for _, args := range m.sel.applyArgs() {
			joined := strings.Join(args, " ")
			stdout, stderr, err := runProvsync(m.bin, args...)
			if err != nil {
				out = append(out, i18n.T(m.lang, "msg.tui.failed", joined, stderr))
				continue
			}
			out = append(out, i18n.T(m.lang, "msg.tui.apply", joined, stdout))
		}
		m.message = out
		m.screen = screenDone
		return tea.Quit()
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
		for _, args := range m.sel.applyArgs() {
			b.WriteString("  provsync " + strings.Join(args, " ") + "\n")
		}
		b.WriteString("\n" + i18n.T(m.lang, "msg.tui.confirmPrompt") + "\n")
	case screenDone:
		b.WriteString(strings.Join(m.message, "\n"))
		b.WriteString("\n\n" + i18n.T(m.lang, "msg.tui.quit") + "\n")
	}
	return b.String()
}
