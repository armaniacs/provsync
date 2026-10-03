# PBI: ツール状態収集を CollectToolStates に共通化する

## 種別
- refactor（挙動不変・最小差分）

## ユーザーストーリー
provsync の保守担当者として、`adapter.Names()` → `adapter.Get()` → `os.Stat()` → `Pull()` の収集手順が `cmdList` / `buildStatusReport` / `doctorCheckTools` の3経路で繰り返されているため、共通コレクタに集約したい、なぜならツール追加・Stat 意味変更・Pull 契約変更のたびに複数箇所の同期が必要になり修正漏れの温床になるから

## 優先度
- 順位: 3
- RICEスコア: 1.33（Reach=5 / Impact=1 / Confidence=0.8 / Effort=3）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticc.md` の候補 C1

## 背景（レビューで確認済みの実在箇所。行番号は概略）
- `internal/cli/status.go` の `cmdList`（概略 18-68 行）: Names を走査し Get→Stat→Pull して provider 数を数える。Get/Pull 失敗時はエラーを返して中断する
- `internal/cli/status.go` の `buildStatusReport`（概略 169-205 行）: tools を走査し Get→Stat→Pull して警告と drift を集計する。Get/Pull 失敗時はエラーを返して中断する
- `internal/cli/doctor.go` の `doctorCheckTools`（概略 138-166 行）: Names を走査し Get→Stat→Pull して診断に分類する。Get 失敗時は skip 継続、Pull 失敗時は NG 項目として継続する（中断しない）
- 軽量版（Get+Stat のみ、Pull なし）は対象外: `internal/cli/help.go` の `printUsage`（概略 94-105 行）、`internal/cli/sync.go` の `detectTools`（概略 128-140 行）、cmdInit のエラー経路（概略 81-87 行）。これらは壊れた設定でも動く必要がある（特に `--help` は設定破損時にこそ必要）ため Pull しない現行意味を維持する

## 改善案
1. `internal/adapter` パッケージに `ToolState` 型（Name / Path / Exists / Providers / Warnings / Err。Err は Get/Pull 失敗時の per-tool エラーで nil 可）と `CollectToolStates(root Root, names []string) []ToolState` を新設する（新規ファイル `internal/adapter/collect.go` を推奨。1 ファイル = 1 責務規約に従う）
2. `cmdList` と `buildStatusReport` はコレクタを使い、per-tool の Err があれば従来どおりエラーを返して中断する（外部挙動の維持）
3. `doctorCheckTools` はコレクタを使い、Err ありを NG 項目に分類する（従来の継続意味の維持）
4. 軽量版 3 箇所は変更しない（理由: Pull しない現行意味の維持。この判断を PBI とコードコメントに記録する）
5. 外部挙動・出力・終了コードは 1 バイトも変えない

## 制約
- 外部挙動を一切変えない（中断 vs 継続の各経路の方針を維持）
- `Adapter` インターフェースのシグネチャを変えない。新設は `ToolState` と `CollectToolStates` のみ
- 既存テストの期待値を書き換えない

## BDD受け入れシナリオ
Scenario: 収集共通化後も status の出力が変わらない
  Given 既存の全テストが通る状態である
  When  3 経路の収集を `CollectToolStates` に寄せる
  Then  全テストが変更なしでパスし、`status` / `list` / `doctor` の出力が変更前と 1 バイトも変わらない

Scenario: 壊れた設定でも --help が動く
  Given JSON として壊れたツール設定がある
  When  `--help` を実行する
  Then  usage が正常に表示される（printUsage は Pull しないため影響を受けない）

Scenario: 壊れたツール設定で doctor が NG を出す
  Given パース不能なツール設定がある
  When  `doctor` を実行する
  Then  該当ツールが NG 項目として報告され、他ツールの診断は継続される（従来どおり中断しない）

## 受け入れ基準
1. `cmdList` / `buildStatusReport` / `doctorCheckTools` が `CollectToolStates` を通る
2. 各経路のエラー方針が維持されている（list/status は中断、doctor は継続分類）
3. 軽量版 3 箇所が変更なしである（理由の記録あり）
4. 全テストが変更なしでパスする
5. `make check` がパスする
6. `go vet` / staticcheck の新規 error が 0 件
7. 出力が byte-identical（`list` / `status` / `status --json` / `doctor` / `--help` が変更前と同じ）

## テスト戦略
- 変更前に現行出力を pin する（`make build` 後に `bin/provsync --root <tmp> list` / `status` / `status --json` / `doctor` / `--help` を保存。doctor NG 経路用に壊れた設定の fixture も用意する）
- 変更後に pin した出力との diff が空であることを確認する
- 既存の cli/adapter テストが変更なしでパスすることを確認する

## 見積もり
3 SP

## Definition of Done
- [x] `ToolState` + `CollectToolStates` が adapter パッケージにある
- [x] 3 経路がコレクタを通り、各々のエラー方針が維持されている
- [x] 軽量版 3 箇所が不変である（理由の記録あり）
- [x] 全テストが変更なしでパスする
- [x] 出力が byte-identical である
- [x] `make check` がパスする

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`internal/adapter/adapter.go` 全体、`internal/cli/status.go`、`internal/cli/doctor.go`、`internal/cli/cli_test.go` の補助関数（setup/mustRun/run）。

### 手順
1. 変更前に現行出力を pin する（正常系 + 壊れた設定の doctor NG + 壊れた設定の --help）
2. `internal/adapter/collect.go` に `ToolState` と `CollectToolStates` を作る
3. 3 経路をコレクタ使用に書き換える（エラー方針は各経路の従来どおり）
4. `go test ./internal/adapter/ ./internal/cli/` が変更なしで通ることを確認する
5. pin した出力との diff が空であることを確認する
6. `make check` を実行する

### 注意
- 担当ファイルは `internal/adapter/collect.go`（新規）+ `internal/cli/status.go` + `internal/cli/doctor.go` のみ。`internal/cli/help.go`・`sync.go`（軽量版）、`internal/fsutil/`、`internal/backup/`、`tui/` には触れない
- 本 PBI は挙動不変の refactor のため、共通ガイドの「テストを先に書く」は pin テスト（現行出力の固定）として適用する
- `make check` が 3 回直しても通らない場合は、00-implementation-guide.md §7 に従い実装を止めて報告する
