# PBI: コマンド定義の単一レジストリへの集約（refactor）

種別: refactor（挙動不変・最小差分）

## ユーザーストーリー
provsync の保守担当者として、コマンドを 1 つ追加するとき Run の switch、helpTexts マップ、commands スライス、cmdCompletion の出力、printUsage のテキストの 5 箇所を手動で揃える必要があるため、単一のレジストリから生成されるようにしたい。揃え忘れは補完やヘルプの欠落として静かに現れるから。

## 優先度
- 順位: 4
- RICEスコア: 0.8（Reach=3 / Impact=1 / Confidence=0.8 / Effort=3）
- 依存: PBI 20（cli.go 責務分割・help.go 生成）の後。PBI 22 と cli_test.go が重なるため直列（PBI 22 の後）
- 台帳: `2026-10-03-00-backlog-holistic.md` の候補 C3

## 背景
コマンド一覧に関わる定義が internal/cli/cli.go 内の複数箇所に分散している（行番号は概略。PBI 20 実施後は helpTexts が help.go に移動されるため、実装時に現位置を確認すること）:

- `RunWith` の switch（旧 cli.go 74-130）でコマンドをハンドラへ対応づけ
- `helpTexts` マップ（旧 1247-1420、約 175 行の文字列）が各コマンドのヘルプを持つ
- `commands` スライス（旧 1450）が補完と検証用の一覧を持つ
- `cmdCompletion`（旧 1421-1448）が commands と `adapter.Names()` から補完スクリプトを生成
- `printUsage`（旧 194-243）が usage テキストをハードコード

新しいコマンドを足すときは上記 5 箇所を手動で揃える必要があり、揃え忘れてもコンパイルは通るため、補完やヘルプの欠落として静かに現れる。

## 改善案
単一のレジストリ構造体スライスを定義し、(a) Run の switch、(b) helpTexts、(c) commands、(d) completion のコマンド一覧をそこから生成する。

```go
type command struct {
    name, summary, usage, help string
    handler func(o *options, args []string) error
    takesArgs bool
}
```

- usage テキストのコマンド一覧節もレジストリから組み立ててよい
- コマンドごとの詳細ヘルプ（helpTexts の内容）はレジストリのフィールドへ移設する

## 制約
- 出力（`--help`、usage、completion、version）を 1 バイトも変えない
- 終了コードも不変
- 未知コマンドのエラーメッセージも不変
- ヘルプテキストの文言は現行のまま移設する（byte-identical）

## BDD受け入れシナリオ
Scenario: 新しいサブコマンドを追加する
  Given 保守担当者が新しいサブコマンドを追加することになった
  When レジストリに 1 エントリ足す
  Then switch・ヘルプ・usage・補完すべてに反映される

Scenario: 既存の全コマンドの出力が変わらない
  Given 既存の全コマンドがある
  When `--help` と `completion zsh` と `completion bash` を実行する
  Then 出力が移動前と 1 バイトも同じである

Scenario: 未知コマンドの扱いが変わらない
  Given 未知のコマンド名がある
  When `provsync <未知のコマンド>` を実行する
  Then エラーメッセージと終了コードが移動前と同じである

## 受け入れ基準
1. 出力が byte-identical（全コマンドの `--help`、引数なし usage、`completion bash|zsh|fish`、`--version`）
2. コマンド一覧の定義が 1 箇所に集約
3. 全テスト変更なしでパス（出力が同じのため）
4. `make check` パス
5. 未知コマンド・使い方エラーの終了コードが不変（0/1/2）
6. README に変更不要（コマンド表は手動管理のまま）で済むことを確認

## テスト戦略
- 移動前に全コマンドの `--help` / usage / completion 出力をファイルに記録して pin する
- 移動後に diff で byte-identical を確認する
- 既存テスト（TestSubcommandHelp, TestCompletion* 等）は変更なしでパスすることを確認する

## 見積もり
3 SP

## Definition of Done
- [ ] 出力が byte-identical（全コマンドの `--help`、引数なし usage、`completion bash|zsh|fish`、`--version`）
- [ ] コマンド一覧の定義が 1 箇所に集約されている
- [ ] 全テストが変更なしでパスする
- [ ] `make check` がパスする
- [ ] 未知コマンド・使い方エラーの終了コードが不変（0/1/2）
- [ ] README に変更不要（コマンド表は手動管理のまま）で済むことを確認した

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/cli/cli.go` の `RunWith` の switch、`helpTexts`、`commands`、`cmdCompletion`、`printUsage` の現位置（PBI 20 実施後は helpTexts が help.go に移動されているため、記載の行番号に頼らず必ず現位置を確認する）。`cli_test.go` の補助関数（`setup` / `mustRun` / `run`）。

### 手順
1. 移動前に全コマンドの `--help`、引数なし usage、`completion bash|zsh|fish`、`--version` の出力をファイルに記録して pin する。
2. レジストリ構造体スライスを定義し、helpTexts の内容を各コマンドの `help` フィールドへ現行の文言のまま移設する。
3. Run の switch、commands スライス、completion のコマンド一覧、usage のコマンド一覧節をレジストリから生成するように変える。
4. pin した出力と diff し、byte-identical であることを確認する。
5. 既存テストを変更せずに `go test ./...` がパスすること、`make check` がパスすることを確認する。

### 注意
- 本 PBI は refactor であり、挙動は一切変えない。出力・終了コード・エラーメッセージ・ヘルプ文言のどれも変更してはならない。
- README は変更不要（コマンド表は手動管理のまま）。CHANGELOG はユーザーから見える変化がないため、記載するとしても [Unreleased] への最小限の 1 行でよい。
