# PBI: internal/cli/cli.go を責務ごとのファイルに分割する

## 種別
- refactor（挙動不変・最小差分。byte 挙動は保ち、ファイル構成だけを変える）

## ユーザーストーリー
provsync の保守担当者として、`internal/cli/cli.go` が 1,605 行の god file（宣言 60 件超）でディスパッチ・コマンドハンドラ・描画・ヘルプ・診断・通信の責務が同居しているため、責務ごとの同一パッケージ内ファイル分割により変更箇所の探索コストを下げたい、なぜなら 1 つの修正のたびに巨大な 1 ファイルを行き来するのは非効率で、新規参加者が関係するコードへたどり着くまでの時間が無駄になるから

## 優先度
- 順位: 1
- RICEスコア: 2.33（Reach=7 / Impact=1 / Confidence=1.0 / Effort=3）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holistic.md` の候補 C1

## 背景（レビューで確認済みの実在箇所。行番号は概略）
`internal/cli/cli.go` は 1,605 行・宣言 60 件超で、次の責務が同居している。

- ディスパッチと flag 解析: `RunWith` / `registerFlags` / `reorder` が冒頭
- 11 コマンドハンドラ: `list` / `status` / `init` / `pull` / `push` / `sync` / `diff` / `undo` / `doctor` / `check` / `completion` が単一ファイル内に直列
- 描画層: `renderStatusText` / `renderPreview` / `renderSemantic` / `driftLines` / `opLabel` / `warnLoosePerm` / `encodeJSON` / `symlinkSuffix`
- ヘルプテキスト: `helpTexts` マップ（約 175 行）+ `cmdCompletion` + `commands` スライス
- doctor 診断群: `cmdDoctor` + `doctorCheck*` 5 関数 + `diagnosisCheck` 型
- レポート型: `listReport` / `statusReport` / `diffReport` / `diagnosisCheck` / `centralInfo` / `toolInfo` / `toolStatus` / `diffChange`
- check 通信: `cmdCheck` / `checkProvider` / `checkTimeout` 変数

## 改善案（同一パッケージ内の機械的移動のみ）
責務ごとに 1 ファイルへ関数・型を cut & paste で移動する。例:

- `help.go`: `helpTexts` + `cmdCompletion` + `commands`
- `render.go`: 描画群 + レポート型
- `doctor.go`: doctor 診断群
- `check.go`: check 通信
- `pipeline.go`: pull/push 前処理

実際の分割単位は実装者が判断してよいが、1 ファイル = 1 責務とする。ロジック・関数シグネチャ・文言は一切変更しない。

## 制約
- 挙動を一切変えない（出力・終了コード・テスト結果は全く同じ）
- ロジックの変更・関数シグネチャの変更をしない（移動のみ）
- `internal/cli/cli_test.go` は変更しなくて済むのが理想（パッケージ内移動のため変更不要のはず）

## BDD受け入れシナリオ
Scenario: 責務分割後も挙動が 1 バイトも変わらない
  Given 既存の全テストが通る状態である
  When  `internal/cli/cli.go` を責務ごとのファイルに分割する
  Then  全テストが変更なしでパスし、`--version` / `--help` / `status` / `list` / `doctor` の出力が移動前と 1 バイトも変わらない

Scenario: 新規貢献者が描画層を修正する
  Given `internal/cli` パッケージが責務ごとに分割されている
  When  描画層を修正するために `render.go` を開く
  Then  描画に関係する関数だけがそこにあり、探索コストが下がっている

## 受け入れ基準
1. 全テストが変更なしでパスする
2. `make check` がパスする
3. `go vet` / staticcheck の新規 error が 0 件
4. 出力が byte-identical（`--version` / `--help` / `status` / `list` / `doctor` の出力が移動前と同じ）
5. cli.go がディスパッチ・共通 option・パイプライン中心になり、700 行未満程度になる
6. 新規ファイルは責務ごとに 1 つである
7. README のパッケージ構成説明に新ファイルを反映する

## テスト戦略
- 移動前に現行出力を pin する（`--version` / `--help` / `status --json` / `doctor` の出力を記録）
- 移動後に byte-identical を確認する（pin した出力との diff が空）
- 単体テストは変更なしで全パスすることを確認する

## 見積もり
3 SP

## Definition of Done
- [x] 全テストが変更なしでパスする
- [x] `make check` がパスする
- [x] `go vet` / staticcheck の新規 error が 0 件
- [x] 出力が byte-identical（`--version` / `--help` / `status` / `list` / `doctor`）
- [x] cli.go が 700 行未満程度になり、ディスパッチ・共通 option・パイプライン中心になっている
- [x] 新規ファイルが責務ごとに 1 つになっている
- [x] README のパッケージ構成説明を更新した

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/cli/cli.go` 全体、`internal/cli/cli_test.go`、README のパッケージ構成説明。

### 手順
1. 移動前に現行出力を pin する: `make build` 後、`bin/provsync --version` / `--help` / `status --json` / `list` / `doctor` の出力をファイルへ保存する。
2. 責務ごとに新ファイルを作り、関数・型をそのまま移動する。変更は import 文の調整のみに限る。
3. cli.go には `Run` / `RunWith` / `options` / `registerFlags` / `reorder` / `applyOrPreview` / pull/push 前処理を残す。
4. `go test ./...` が変更なしで通ることを確認する。
5. pin した出力との diff が空（byte-identical）であることを確認する。
6. `make check` を実行する。
7. README のパッケージ構成説明に新ファイルを追記する。

### 注意
- 本 PBI は挙動不変の refactor のため、共通ガイドの「テストを先に書く」は適用しない。既存テストが仕様であり、新規テストは追加しない。
- 移動以外の変更（命名変更・再フォーマット・コメント整理）をしない。`gofmt` が通る状態だけを保つ。
- `make check` が 3 回直しても通らない場合、または cli_test.go を書き換えないと通らない場合は、共通ガイド §7 に従い実装を止めて報告する。
