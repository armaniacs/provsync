# PBI: TUI と実バイナリの連携契約を統合テストで固定する

## 種別
- test（本番コードの変更なし。モジュール境界の契約をテストで固定する）

## ユーザーストーリー
provsync-tui の保守担当者として、TUI が組み立てる子プロセス引数（`push <tool> --provider <p> --write`）が fakeBin（シェルスタブ）相手の単体テストでしか検証されていないため、実バイナリ相手の統合テストで契約を固定したい、なぜなら CLI 側が push の引数体系を変えたら TUI の適用が実行時に壊れ、コンパイル時・単体テストでは検出できないから

## 優先度
- 順位: 4
- RICEスコア: 0.8（Reach=3 / Impact=1 / Confidence=0.8 / Effort=3）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticc.md` の候補 C2

## 背景（レビューで確認済みの実在箇所。行番号は概略）
- `tui/model.go` の `applyArgs` が `push` / ツール名 / `--provider` / provider 名 / `--write` の引数列を文字列結合で組み立てる。`TestApplyArgsBuildsPushCommands`（`tui/model_test.go` 概略 93 行目付近）はこの文字列を assertion するのみで、相手側（実バイナリ）が受け付けるかは未検証
- `tui/i18n_test.go` の `fakeBin`（概略 102 行目付近）は引数によらず固定内容を返すシェルスタブで、`fetchStatus` / `runProvsync` の全テストがこれを相手にしている
- TUI は独立モジュール（`tui/go.mod` が `replace github.com/armaniacs/provsync => ../` で親を参照）で、親バイナリをテスト内でビルドできる

## 改善案
1. `tui/` に実連携テスト（新規ファイル推奨。例: `tui/e2e_contract_test.go`）を追加する
2. テスト内で親モジュールの provsync バイナリを temp ディレクトリへビルドする（`go build`。CI に Go がある前提）
3. `t.Setenv` で `PROVSYNC_BIN`（ビルドした実バイナリ）と `HOME`（temp。XDG 系も必要なら temp に）を差し替え、ツール設定（kilocode JSONC）と中央設定の drift がある状態を作る
4. `fetchStatus(bin)` → `newSelection` → 選択 → `applyArgs` → `runProvsync` の全路を実ファイル相手に実行し、ツール設定ファイルが期待どおり更新されることを検証する
5. 本番コード（`tui/*.go` の非テストファイル）・親モジュール・go.mod は一切変更しない

## 制約
- 本番コードを変更しない（テストファイルの追加のみ）
- 実ホーム・実設定ファイルを触らない（`HOME`/`PROVSYNC_BIN` は temp に差し替え、`t.TempDir()` と `t.Setenv` のみを使う）
- `tui/go.mod` の依存を追加・変更しない
- 既存テストの期待値を書き換えない

## BDD受け入れシナリオ
Scenario: 実バイナリ相手に適用が通る
  Given temp HOME に drift ありのツール設定と中央設定があり、実バイナリを PROVSYNC_BIN に向けてある
  When  fetch→選択→applyArgs→runProvsync の全路を実行する
  Then  子プロセスが成功し、ツール設定ファイルが中央設定どおりに更新される

Scenario: push の引数体系が変わったら検出できる
  Given 実連携テストがある
  When  CLI 側の push が `--provider` を受け付けなくなったとする
  Then  実連携テストが失敗する（文字列 assertion の単体テストだけでは検出できない破損を拾う）

## 受け入れ基準
1. 実バイナリ相手の連携テストが存在し、パスする
2. テストが実ホーム・実設定ファイルに触れない（temp + Setenv のみ）
3. 既存の tui テストが変更なしでパスする
4. `make tui-check` がパスする
5. `go test -race`（tui モジュール）がパスする
6. 本番コード・go.mod に差分がない

## テスト戦略
- 新規テストは通常の `go test` フローで実行できること（手動のバイナリ配置を要求しない。テスト内で `go build` する）
- 親バイナリのビルド失敗時はテストをスキップではなく失敗させる（契約検証が素通りしないようにする）
- ネットワーク・実ホームへの依存を持たせない

## 見積もり
3 SP

## Definition of Done
- [ ] 実連携テストが存在しパスする
- [ ] temp + Setenv のみで実環境に触れない
- [ ] 既存テストが変更なしでパスする
- [ ] `make tui-check` がパスする
- [ ] `go test -race`（tui モジュール）がパスする
- [ ] 本番コード・go.mod に差分がない

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`tui/model.go` 全体、`tui/tui.go` の `applySelected` 付近、`tui/i18n_test.go` の `fakeBin`、`tui/model_test.go` の `TestApplyArgsBuildsPushCommands`、`tui/go.mod`。

### 手順
1. 実連携テストを書く（親ビルド→env 差し替え→fixture 作成→全路実行→ファイル検証）
2. `cd tui && go test ./...` がパスすることを確認する
3. `cd tui && go test -race ./...` を実行する
4. `make tui-check` を実行する
5. `git status` で本番コード・go.mod に差分がないことを確認する

### 注意
- 担当ファイルは `tui/` 配下の新規テストファイルのみ。`tui/*.go` の本番コード、`internal/` 配下、`pbi/` 配下には触れない
- テスト内での `go build` は `t.TempDir()` へ出力し、並列実行時の衝突を避ける
- `make tui-check` が 3 回直しても通らない場合は、00-implementation-guide.md §7 に従い実装を止めて報告する
