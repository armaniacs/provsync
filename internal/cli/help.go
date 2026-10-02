// Package cli のうち、ヘルプ表示とシェル補完を担う。
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/version"
)

// printUsage は使い方と、読み書きする設定ファイルの場所を出す。
// HOME が解決できない場合でも使い方本文は出し、設定ファイル節だけ省略する。
func printUsage(o *options) {
	fmt.Fprintln(o.out, "provsync "+version.String())
	fmt.Fprintln(o.out, `使い方: provsync <command> [flags]

コマンド:
  list                     対応ツールと設定パスを表示
  status [tool...]         ツールと中央設定の同期状態を表示
  init [tool]              初回セットアップ(中央設定を作る)
  pull <tool>              ツール設定を中央設定へ取り込む
  push <tool>              中央設定をツール設定へ反映する
  sync --from <a> --to <b> a を取り込み b へ反映する(--from 省略可)
  diff <from> <to>         from を to に適用した場合の差分を表示
  undo [id]                直前または指定操作を復元する(--list で履歴)
  doctor                   環境を診断する(通信しない)
  check                    各 provider の API 到達可否を確認する(明示実行のみ)

共通フラグ:
  --write          変更を書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)
  --no-backup      バックアップを記録しない
  --root <dir>     パス解決の基準を差し替える(テスト用)`)

	root, err := o.root()
	if err != nil {
		return
	}
	fmt.Fprintln(o.out, "\n設定ファイル:")
	central := root.CentralConfigPath()
	if _, err := os.Stat(central); err == nil {
		fmt.Fprintf(o.out, "  中央設定: %s\n", central)
	} else {
		fmt.Fprintf(o.out, "  中央設定: %s (未作成)\n", central)
		fmt.Fprintln(o.out, "    provsync init <tool> --write で作成します")
	}
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			continue
		}
		if _, err := os.Stat(a.Path()); err == nil {
			fmt.Fprintf(o.out, "  %-9s %s\n", name, a.Path())
		} else {
			fmt.Fprintf(o.out, "  %-9s %s (未作成)\n", name, a.Path())
		}
	}
	fmt.Fprintf(o.out, "  バックアップ: %s\n", root.StateDir())
}

// helpTexts はサブコマンド別のヘルプ。`provsync <cmd> --help` で出す。
var helpTexts = map[string]string{
	"list": `list - 対応ツールと設定パスを表示

用途: 各ツールの設定ファイルと中央設定のパス、provider 数を表示する。

使い方: provsync list

例:
  provsync list
`,
	"status": `status - ツールと中央設定の同期状態を表示

用途: ツール設定と中央設定の provider 差分( drift )を表示する。

使い方: provsync status [tool...]

引数: tool を省略すると全対応ツールを表示する。

関連フラグ:
  --json    JSON で出力する

例:
  provsync status
  provsync status kilocode
`,
	"init": `init - 初回セットアップ(中央設定を作る)

用途: ツール設定から中央設定を作成する。初回専用で、既存の中央設定は上書きしない。

使い方: provsync init [tool]

引数: tool を省略すると、設定ファイルが存在するツールを検出する。
      候補が複数ある場合は一覧を表示するので指定する。

関連フラグ:
  --write   中央設定を作成する(既定はプレビュー)

例:
  provsync init kilocode
  provsync init kilocode --write
`,
	"pull": `pull - ツール設定を中央設定へ取り込む

用途: ツール設定の provider エントリを中央設定へマージする。

使い方: provsync pull <tool>

引数: tool は kilocode(別名 kilo) / opencode。

関連フラグ:
  --write          中央設定へ書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync pull kilocode
  provsync pull kilocode --write
`,
	"push": `push - 中央設定をツール設定へ反映する

用途: 中央設定の provider エントリをツール設定へマージする。

使い方: provsync push <tool>

引数: tool は kilocode(別名 kilo) / opencode。

関連フラグ:
  --write          ツール設定へ書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync push opencode
  provsync push opencode --write
`,
	"sync": `sync - 取り込みと反映を一度に行う

用途: --from のツール設定を中央設定へ取り込み、--to のツール設定へ反映する。

使い方: provsync sync --from <a> --to <b>

引数: --from を省略すると中央設定をそのまま使う。

関連フラグ:
  --from <tool>   取り込み元ツール(省略可)
  --to <tool>     反映先ツール(必須)
  --write         ファイルへ書き込む(既定はプレビュー)

例:
  provsync sync --from kilocode --to opencode --write
`,
	"diff": `diff - 適用した場合の差分を表示

用途: from を to に適用した場合の意味差分と統合 diff を表示する。

使い方: provsync diff <from> <to>

引数: from / to はツール名か central。

関連フラグ:
  --show-secrets   秘密の値をそのまま表示する(非推奨)

例:
  provsync diff kilocode opencode
`,
	"undo": `undo - 直前または指定操作を復元する

用途: 書き込み操作をバックアップから復元する。--write は不要で直接適用する。

使い方: provsync undo [id]

引数: id を省略すると直前の書き込みを復元する。undo 自体を undo できる(redo)。

関連フラグ:
  --list   履歴を表示する
  --prune / --keep <n>   履歴を掃除する

例:
  provsync undo
  provsync undo --list
  provsync undo 20261002T093045-8c2d
`,
	"doctor": `doctor - 環境を診断する

用途: 設定の存在・構文・apiKeyEnv の環境変数・権限を一覧で診断する。

使い方: provsync doctor

通信せず、ファイルも書かない。NG があるときは終了コード 1 で終わる。

例:
  provsync doctor
`,
	"check": `check - API の疎通確認

用途: 中央設定の各 provider について API への到達可否を確認する。

使い方: provsync check

通信するのはこのコマンドだけ。他のコマンドから呼ばない。
タイムアウトは 1 provider あたり 5 秒。秘密の値は出力しない。

関連フラグ:
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync check
`,
	"completion": `completion - シェル補完スクリプトを出力

用途: bash / zsh / fish 用の補完スクリプトを標準出力へ出す。

使い方: provsync completion <shell>

引数: shell は bash / zsh / fish。

例:
  # zsh
  provsync completion zsh > "${fpath[1]}/_provsync"

  # bash
  source <(provsync completion bash)
`,
	"version": `version - バージョンを表示

用途: バイナリのバージョンを 1 行で表示する。

使い方: provsync version

例:
  provsync version
  provsync --version
`,
}

// cmdCompletion はシェル補完スクリプトを出力する。
func cmdCompletion(o *options, args []string) error {
	if len(args) != 1 {
		return usageErr("使い方: provsync completion <bash|zsh|fish>")
	}
	shell := args[0]
	tools := adapter.Names()
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
		return usageErr("未知のシェル %q です(有効: bash / zsh / fish)", shell)
	}
	return nil
}

var commands = []string{"list", "status", "init", "pull", "push", "sync", "diff", "undo", "doctor", "check", "completion", "version"}
