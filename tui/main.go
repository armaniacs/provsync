// Package main は provsync の TUI ダッシュボード。
// コアの go.mod に bubbletea などの依存を持ち込まないため、独立したモジュールに
// 隔離されている。状態取得と適用は provsync バイナリを子プロセスとして呼ぶ方式で、
// 変更内容を自前で計算しない。メッセージカタログは親モジュールの internal/i18n を
// replace 指令で共用する。
package main

import (
	"fmt"
	"os"

	"github.com/armaniacs/provsync/internal/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func main() {
	// 言語判定はプロセス入口で 1 回。子プロセスの provsync も同じ env から
	// 同じ言語を解決するため、表示が混在しない。
	lang := i18n.ResolveFromEnv()
	if !isInteractive(os.Stdin) {
		fmt.Fprintln(os.Stderr, i18n.T(lang, "err.tui.interactive"))
		os.Exit(1)
	}
	bin := os.Getenv("PROVSYNC_BIN")
	if bin == "" {
		bin = "provsync"
	}
	report, err := fetchStatus(bin)
	if err != nil {
		// fetchStatus のエラーは言語中立の Message。ここで初めて言語が決まる。
		fmt.Fprintln(os.Stderr, "provsync-tui:", i18n.Localize(lang, err))
		os.Exit(1)
	}
	m := newListModel(bin, report, lang)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "provsync-tui:", i18n.Localize(lang, err))
		os.Exit(1)
	}
}

// isInteractive は f が端末かを返す。非対話端末では TUI を起動しない。
// os.ModeCharDevice では /dev/null も真になるため、term.IsTerminal を使う。
func isInteractive(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
