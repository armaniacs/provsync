// Package cli のうち、ヘルプ表示とシェル補完を担う。
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
)

// command はサブコマンド 1 件の定義。コマンド一覧はこのレジストリに集約し、
// dispatch・ヘルプ・usage・補完がここから生成される。新しいコマンドは
// commandRegistry に 1 エントリ足すだけで反映される。文言はカタログ ID で持ち、
// 表示時に初めて言語が決まる。
type command struct {
	name       string
	usage      string // usage のコマンド部分(例: "status [tool...]")。構文は言語中立
	summaryKey string // usage のコマンド一覧の説明(カタログ ID)
	helpKey    string // 詳細ヘルプ(`provsync <cmd> --help`)の本文(カタログ ID)
	handler    func(o *options, args []string) error
	// inUsage は usage のコマンド一覧に載せるか。completion / version のような
	// meta コマンドは従来どおり一覧から除外する。
	inUsage bool
}

// commandRegistry はサブコマンドの一覧を返す。関数にしているのは、ハンドラが
// レジストリ自身を参照する(補完がコマンド一覧を使う)ため、パッケージレベルの
// 初期化サイクルを避けるためである。
func commandRegistry() []command {
	return []command{
		{name: "list", usage: "list", summaryKey: "usage.summary.list", helpKey: "help.list", handler: cmdList, inUsage: true},
		{name: "status", usage: "status [tool...]", summaryKey: "usage.summary.status", helpKey: "help.status", handler: cmdStatus, inUsage: true},
		{name: "init", usage: "init [tool]", summaryKey: "usage.summary.init", helpKey: "help.init", handler: cmdInit, inUsage: true},
		{name: "pull", usage: "pull <tool>", summaryKey: "usage.summary.pull", helpKey: "help.pull", handler: cmdPull, inUsage: true},
		{name: "push", usage: "push <tool>", summaryKey: "usage.summary.push", helpKey: "help.push", handler: cmdPush, inUsage: true},
		{name: "sync", usage: "sync --from <a> --to <b>", summaryKey: "usage.summary.sync", helpKey: "help.sync", handler: cmdSync, inUsage: true},
		{name: "diff", usage: "diff <from> <to>", summaryKey: "usage.summary.diff", helpKey: "help.diff", handler: cmdDiff, inUsage: true},
		{name: "undo", usage: "undo [id]", summaryKey: "usage.summary.undo", helpKey: "help.undo", handler: cmdUndo, inUsage: true},
		{name: "cleanup", usage: "cleanup [--write] [--yes]", summaryKey: "usage.summary.cleanup", helpKey: "help.cleanup", handler: cmdCleanup, inUsage: true},
		{name: "doctor", usage: "doctor", summaryKey: "usage.summary.doctor", helpKey: "help.doctor", handler: cmdDoctor, inUsage: true},
		{name: "check", usage: "check", summaryKey: "usage.summary.check", helpKey: "help.check", handler: cmdCheck, inUsage: true},
		{name: "completion", usage: "completion <shell>", summaryKey: "usage.summary.completion", helpKey: "help.completion", handler: cmdCompletion},
		{name: "version", usage: "version", summaryKey: "usage.summary.version", helpKey: "help.version", handler: cmdVersion},
	}
}

// commandNames はレジストリのコマンド名一覧を返す。補完で使う。
func commandNames() []string {
	names := make([]string, 0, len(commandRegistry()))
	for _, c := range commandRegistry() {
		names = append(names, c.name)
	}
	return names
}

// lookupCommand は名前でレジストリを引く。
func lookupCommand(name string) (command, bool) {
	for _, c := range commandRegistry() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// printUsage は使い方と、読み書きする設定ファイルの場所を出す。
// HOME が解決できない場合でも使い方本文は出し、設定ファイル節だけ省略する。
func printUsage(o *options) {
	var b strings.Builder
	b.WriteString(o.T("usage.header"))
	b.WriteString("\n")
	for _, c := range commandRegistry() {
		if !c.inUsage {
			continue
		}
		fmt.Fprintf(&b, "  %-24s %s\n", c.usage, o.T(c.summaryKey))
	}
	b.WriteString(o.T("usage.commonFlags"))
	printVersion(o.out)
	fmt.Fprintln(o.out, b.String())

	root, err := o.root()
	if err != nil {
		return
	}
	fmt.Fprintln(o.out, "\n"+o.T("usage.filesHeader"))
	central := root.CentralConfigPath()
	if _, err := os.Stat(central); err == nil {
		o.msgf(o.out, "usage.central", central)
	} else {
		o.msgf(o.out, "usage.centralMissing", central)
		o.msgf(o.out, "usage.createdBy")
	}
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			continue
		}
		if _, err := os.Stat(a.Path()); err == nil {
			fmt.Fprintf(o.out, "  %-9s %s\n", name, a.Path())
		} else {
			o.msgf(o.out, "usage.toolMissing", name, a.Path())
		}
	}
	o.msgf(o.out, "usage.backupDir", root.StateDir())
}

// cmdCompletion はシェル補完スクリプトを出力する。
// スクリプト本体は機械可読のため翻訳しない。使い方エラーのみ翻訳される。
func cmdCompletion(o *options, args []string) error {
	if len(args) != 1 {
		return o.usageErr("err.usage.completion")
	}
	shell := args[0]
	tools := adapter.Names()
	commands := commandNames()
	switch shell {
	case "bash":
		fmt.Fprintf(o.out, `_provsync() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
  else
    COMPREPLY=( $(compgen -W "%s --write --provider --no-backup --show-secrets --json --exit-code" -- "$cur") )
  fi
}
complete -F _provsync provsync
`, strings.Join(commands, " "), strings.Join(tools, " "))
	case "zsh":
		fmt.Fprintf(o.out, "#compdef provsync\n\n_provsync() {\n  _arguments '1: :(%s)' '*: :(%s)'\n}\ncompdef _provsync provsync\n", strings.Join(commands, " "), strings.Join(tools, " "))
	case "fish":
		fmt.Fprintf(o.out, "complete -c provsync -n '__fish_use_subcommand' -a '%s'\n", strings.Join(commands, " "))
		fmt.Fprintf(o.out, "complete -c provsync -n '__fish_seen_subcommand_from pull push' -a '%s'\n", strings.Join(tools, " "))
	default:
		return o.usageErr("err.completion.unknownShell", shell)
	}
	return nil
}
