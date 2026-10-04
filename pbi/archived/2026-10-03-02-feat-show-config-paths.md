# PBI: 設定ファイルの場所を表示する

## ユーザーストーリー
provsync の利用者として、引数なし実行で中央設定と各ツール設定のファイルパスを見たい、なぜなら「どのファイルが読み書きされるか」が分からないと、安心して `--write` できないから

## 優先度
- 順位: 02 / 19
- RICEスコア: 40（Reach=10 / Impact=1 / Confidence=100% / Effort=0.25）
- 根拠: 01 と同じ引数なし出力を触るため 01 の直後に行う。`list` に既にある情報の再利用で低コスト

## BDD受け入れシナリオ
Scenario: 引数なし実行で設定パスが表示される
  Given HOME 配下に中央設定が存在する
  When  `provsync` を引数なしで実行する
  Then  バージョンに続けて、中央設定のパスと各ツール（kilocode / opencode）の設定パスが表示される

Scenario: 未作成のファイルは未作成と分かる
  Given 中央設定がまだ作られていない
  When  `provsync` を引数なしで実行する
  Then  中央設定のパスの横に「未作成」と表示され、`provsync init` への案内が出る

Scenario: --root 指定時は差し替え後のパスを表示する
  Given `--root /tmp/x` を指定する
  When  引数なしまたは `--help` で実行する
  Then  `/tmp/x` 基準で解決したパスが表示される

## 受け入れ基準
- [x] 引数なしと `--help` の双方でバージョン・中央設定・各ツールのパス・バックアップ保存先を表示する
- [x] 存在有無（未作成）を併記する。ファイルの中身は読まず、provider 数は出さない（高速・安全のため `list` に任せる）
- [x] 秘密情報を出力しない
- [x] パス表示は `adapter.Root` の解決結果を唯一の情報源とする（cli 側で再計算しない）

## テスト戦略
- E2E: 実バイナリを HOME を差し替えて実行し出力を確認
- 統合: `cli.Run --root <tmpdir>` で存在あり / なしの両出力
- 単体: パス表示の整形関数

## 見積もり
1 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/cli/cli.go` の `Run`（`opts.help || len(pos) == 0` の分岐）と `usage`、`cmdList`（パスの出し方の手本）、`internal/adapter/adapter.go` の `Root` / `Names` / `Get`。01 が済んでいること。

### 手順
1. `usage(out io.Writer)` を `printUsage(o *options)` に置き換える（呼び出しは `Run` 内の 1 か所）。中で次を順に出す: バージョン行、従来の使い方本文、空行、設定ファイル節。
2. 設定ファイル節の作り方:
   - `root, err := o.root()`。エラーなら設定ファイル節だけ省略し、使い方本文は出す（HOME が無くても `--help` は動く）。
   - 出力する行（パスは `adapter` の解決結果のみ。cli 側で `filepath.Join` しない）:
     - `中央設定: <root.CentralConfigPath()>` ＋ 存在しなければ ` (未作成)`
     - 各ツール: `for _, name := range adapter.Names()` → `a, _ := adapter.Get(name, root)` → `<name> <a.Path()>` ＋ ` (未作成)`
     - `バックアップ: <root.StateDir()>`
   - 存在確認は `os.Stat` のみ。ファイルの中身は読まない（provider 数も出さない）。
3. 中央設定が未作成のとき、案内を 1 行足す。init コマンド（05）が未実装の間は `provsync pull <tool> --write で作成します`、05 の実装時に `provsync init <tool> --write で作成します` へ書き換える。
4. テスト（`cli_test.go`）:
   - `TestNoArgsShowsConfigPaths`: `setup(t)` 後、引数なしで実行 → 出力に `f.kilo`、`f.opencode`、`f.central`、`未作成`（中央は未作成）が含まれる。
   - `TestHelpFlagShowsConfigPaths`: `--help` でも同じ。
   - `TestNoArgsAfterPullShowsCentralExists`: `pull kilocode --write` 後は中央設定の行に `未作成` が付かない。

### 注意
- `--root` を渡したテストでは、その配下のパスが出る（`ResolveRoot` が処理する）。
- 秘密や設定の中身を出さない。パスのみ。
