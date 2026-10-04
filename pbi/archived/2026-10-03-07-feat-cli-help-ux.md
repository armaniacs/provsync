# PBI: CLI のヘルプ・終了コード・シェル補完を整える

## ユーザーストーリー
provsync の利用者として、サブコマンドごとのヘルプと一貫した終了コードがほしい、なぜならスクリプトや CI から安全に呼べ、使い方も `--help` だけで分かるようにしたいから

## 優先度
- 順位: 07 / 19
- RICEスコア: 8（Reach=10 / Impact=1 / Confidence=80% / Effort=1）
- 根拠: 利用者体験とスクリプト連携の品質向上。init（05）追加後にサブコマンドが出揃ってから整えると手戻りが少ない

## BDD受け入れシナリオ
Scenario: サブコマンドのヘルプ
  Given provsync をビルド済みである
  When  `provsync pull --help` を実行する
  Then  pull の用途・引数・関連フラグ・使用例が表示され、終了コードは 0 である

Scenario: 使い方の誤りは終了コード 2
  Given 未知のコマンド名がある
  When  `provsync frobnicate` を実行する
  Then  エラーは stderr に出て、終了コードは 2 である

Scenario: 実行時エラーは終了コード 1
  Given 壊れた JSON の設定ファイルがある
  When  `provsync pull kilocode` を実行する
  Then  エラーは stderr に出て、終了コードは 1 である

Scenario: シェル補完を生成する
  Given zsh を使っている
  When  `provsync completion zsh` を実行する
  Then  サブコマンド・ツール名・フラグを補完する zsh スクリプトが標準出力に出る

## 受け入れ基準
- [x] 各サブコマンドが専用の usage を持つ
- [x] 使い方エラー=2、実行時エラー=1、成功=0 に統一（main.go で型付きエラーを判別）
- [x] 通常出力は stdout、エラーと警告は stderr
- [x] `completion bash|zsh|fish` を標準ライブラリのみで実装
- [x] ツール名の綴り間違いに「もしかして」を提示（任意）
- [x] `NO_COLOR` を尊重する（色を使う場合のみ）

## テスト戦略
- E2E: 実バイナリで終了コードと stdout/stderr の分離を確認
- 統合: `cli.Run` の各ヘルプ出力、エラー種別
- 単体: エラー型から終了コードへの対応

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`main.go`、`internal/cli/cli.go` の `Run` / `usage` / `reorder` と `fmt.Fprintf(o.out, "警告: ...")` の箇所、`cli_test.go` の `run` 補助関数。05（init）が済んでいること（サブコマンドが出揃ってから）。

### 設計の確認
- `cli.Run(args, out)` のシグネチャは変えない（既存テストが使う）。標準エラー出力用に `RunWith(args, out, errOut)` を新設し、`Run` は `RunWith(args, out, out)` とする。こうすると既存テストは変更不要。
- 終了コード: 0 成功 / 1 実行時エラー / 2 使い方の誤り。

### 手順
1. 型付きエラーを `internal/cli/errors.go` に作る。

```go
package cli

// UsageError はコマンドの使い方の誤りを表す。main が終了コード 2 に対応づける。
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }
```
   未知のコマンド、位置引数の数の誤り（`使い方: provsync pull <tool>` など）をすべて `&UsageError{...}` で返すように変える。`fmt.Errorf` のままの箇所は実行時エラー（終了コード 1）。
2. `options` に `errOut io.Writer` を足し、`RunWith` で設定する。`"警告: ..."` 系の出力（`cmdPull`・`cmdStatus`・`buildSyncPlan`・init の警告）を `o.errOut` へ変える。
3. `main.go`:

```go
err := cli.RunWith(os.Args[1:], os.Stdout, os.Stderr)
if err != nil {
	fmt.Fprintln(os.Stderr, "provsync:", err)
	var ue *cli.UsageError
	if errors.As(err, &ue) {
		os.Exit(2)
	}
	os.Exit(1)
}
```
4. サブコマンド別ヘルプ: `var helpTexts = map[string]string{"pull": "...", ...}` を作る（用途・引数・フラグ・使用例を日本語で各 5〜10 行）。`Run` の分岐を「`opts.help` かつ `len(pos) > 0` かつ `helpTexts[pos[0]]` がある → そのヘルプを出して `return nil`」「それ以外の `opts.help || len(pos) == 0` → 従来の usage」の順にする。
5. 補完: `completion bash|zsh|fish` を `cmdCompletion` で実装。コマンド一覧は `commands` スライス 1 つから生成し、スクリプト本文は文字列テンプレートにする。

```go
var commands = []string{"list", "status", "pull", "push", "sync", "diff", "undo", "init", "completion", "version"}
```
   bash の最小形:
```bash
_provsync() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "<commands>" -- "$cur") )
  else
    COMPREPLY=( $(compgen -W "<tools> --write --provider --no-backup" -- "$cur") )
  fi
}
complete -F _provsync provsync
```
   zsh は `#compdef provsync` と `_arguments '1: :(<commands>)' '*: :(<tools>)'`、fish は `complete -c provsync -n '__fish_use_subcommand' -a '<commands>'`。`<tools>` は `adapter.Names()` から埋める。
6. テスト: 未知コマンドが `*UsageError`（`errors.As`）、壊れた JSON の pull が `*UsageError` ではない、`pull --help` の出力に `pull` と `--write`、`completion zsh` の出力に `compdef` と `kilocode`、警告が `errOut` 側に出る（`RunWith` を直接呼び、2 つのバッファを使う）。
7. 終了コードの E2E: `go build -o /tmp/provsync-x . && /tmp/provsync-x frobnicate; echo $?` が 2。

### 注意
- 通常出力（プレビュー・diff・status 本体）は stdout のまま。警告とエラーだけ stderr。
- 既存テストが警告を stdout の文字列で検査している場合、`Run` は errOut=out なので変更不要のはず。壊れたら止めて報告。
