// Package main は provsync の TUI ダッシュボード。
// コアの go.mod に依存を持ち込まないため、独立したモジュールに隔離されている。
// 状態取得と適用は provsync バイナリを子プロセスとして呼ぶ方式で、
// 変更内容を自前で計算しない。
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func main() {
	if !isInteractive(os.Stdin) {
		fmt.Fprintln(os.Stderr, "対話が必要です(標準入力が端末ではありません)")
		os.Exit(1)
	}
	bin := os.Getenv("PROVSYNC_BIN")
	if bin == "" {
		bin = "provsync"
	}
	report, err := fetchStatus(bin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "provsync-tui:", err)
		os.Exit(1)
	}
	m := newListModel(bin, report)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "provsync-tui:", err)
		os.Exit(1)
	}
}

// isInteractive は f が端末かを返す。非対話端末では TUI を起動しない。
// os.ModeCharDevice では /dev/null も真になるため、term.IsTerminal を使う。
func isInteractive(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
