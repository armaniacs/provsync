package main

import (
	"strings"

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
	report  statusReport
	sel     *selection
	cursor  int
	screen  screen
	message []string
}

func newListModel(bin string, report statusReport) *listModel {
	return &listModel{
		bin:    bin,
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
func (m *listModel) applySelected() tea.Cmd {
	return func() tea.Msg {
		var out []string
		for _, args := range m.sel.applyArgs() {
			stdout, stderr, err := runProvsync(m.bin, args...)
			if err != nil {
				out = append(out, "失敗: provsync "+strings.Join(args, " ")+": "+stderr)
				continue
			}
			out = append(out, "適用: provsync "+strings.Join(args, " ")+"\n"+stdout)
		}
		m.message = out
		m.screen = screenDone
		return tea.Quit()
	}
}

func (m *listModel) View() string {
	var b strings.Builder
	b.WriteString("provsync-tui\n\n")
	b.WriteString("中央設定: " + m.report.Central.Path + "\n\n")

	switch m.screen {
	case screenList:
		if len(m.sel.Items) == 0 {
			b.WriteString("適用候補となる差分がありません(同期済み)\n\n")
			b.WriteString("q で終了\n")
			return b.String()
		}
		b.WriteString("スペースで選択、Enter で確認、q で終了\n\n")
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
		b.WriteString("次のコマンドを実行します(--write 付き):\n\n")
		for _, args := range m.sel.applyArgs() {
			b.WriteString("  provsync " + strings.Join(args, " ") + "\n")
		}
		b.WriteString("\n実行しますか? y / n\n")
	case screenDone:
		b.WriteString(strings.Join(m.message, "\n"))
		b.WriteString("\n\nq で終了\n")
	}
	return b.String()
}
