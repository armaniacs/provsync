# PBI: status --json の drift 構造化（driftEntries）と TUI の逆解析依存の解消

## ユーザーストーリー
TUI 利用者として、`status --json` の drift を人間可読文字列の逆解析に頼らず構造化データとして受け取りたい、なぜなら `provsync-tui` は現在 `status --json` の人間可読な drift 文字列（`"xxx: 中央に無い"` 等）を `": "` で逆解析して provider 名を復元しており、CLI 側の文言変更で TUI が静かに壊れる（候補が空になるだけでエラーが出ない）から

## 優先度
- 順位: 3
- RICE スコア: 1.6（Reach=3 / Impact=2 / Confidence=80% / Effort=3）
- 依存: PBI 21（パイプライン共通化）の後（推奨）
- 台帳: `2026-10-03-00-backlog-holistic.md` の候補 C4
- 種別: feat（外部挙動への additive 変更。2026-10-03 の候補確認で「C4 も含める」とユーザー承認済み）

## 背景
- `driftLines`（`internal/cli/cli.go`、旧 489-496 付近）が人間可読文字列を生成する: `%s: ツールに無い` / `%s: 中央に無い` / `%s: 差分あり` / 差分なし時は単独の `差分なし`
- `status --json` では `toolStatus.Drift []string` にそのまま文字列が入る（`buildStatusReport` で「差分なし」のみ除外）
- `tui/model.go` の `providerFromDriftLine`（旧 95-101 付近）が `strings.Index(line, ": ")` で逆解析しており、CLI 側の文言変更で TUI が静かに壊れる
- `provsync-tui` は別モジュール（`tui/go.mod`）でコアの internal を import できないため、最小限の JSON 構造を tui 側で定義している（`tui/model.go` の `statusReport`）

## BDD受け入れシナリオ

Scenario: status --json に driftEntries が additive に現れる
  Given 中央にのみ存在する provider "x" がある
  When  `provsync status --json` を実行する
  Then  該当ツールの driftEntries に `{"provider": "x", "op": "not-in-central"}` が含まれ、既存の drift 文字列は従来どおりである

Scenario: TUI は driftEntries を優先して候補を組み立てる
  Given TUI が `status --json` の応答を読み込む
  When  provider 候補を組み立てる
  Then  応答に driftEntries がある限り、drift 文字列の逆解析に頼らず driftEntries から候補を組み立てる

Scenario: driftEntries が無い応答では従来の逆解析にフォールバックする
  Given driftEntries を含まない旧 CLI の `status --json` 応答
  When  TUI が provider 候補を組み立てる
  Then  従来の `providerFromDriftLine` による逆解析で候補を組み立てる

## 受け入れ基準
- [ ] `status --json` に driftEntries が additive で追加される（既存の `drift` は削除しない）
- [ ] 既存の drift 文字列・テキスト出力は byte-identical である
- [ ] TUI は driftEntries を優先使用する
- [ ] TUI の旧フォールバック（`providerFromDriftLine`）は後方互換のために残る
- [ ] op 値の集合（`not-in-tool` / `not-in-central` / `drift`）が README に記載される
- [ ] CHANGELOG `[Unreleased]` に追記する
- [ ] `make check` がパスする
- [ ] tui モジュールの `go build ./...` / `go test ./...` が tui/ 内でパスする

## テスト戦略
- `internal/cli/cli_test.go` に driftEntries の新テストを追加する（既存の `TestStatusJSON` は変更しない）
- `tui/model_test.go` に driftEntries 優先のテストとフォールバックのテストを追加する
- テキスト出力の parity は既存テストで確認する

## 見積もり
3 SP

## Definition of Done
- [ ] `status --json` に driftEntries が additive で追加されている
- [ ] 既存の drift 文字列・テキスト出力が byte-identical である
- [ ] TUI が driftEntries を優先使用する
- [ ] TUI の旧フォールバック（`providerFromDriftLine`）が後方互換のために残っている
- [ ] op 値の集合が README に記載されている
- [ ] CHANGELOG `[Unreleased]` に追記した
- [ ] `make check` がパスする
- [ ] tui モジュールの `go build ./...` / `go test ./...` が tui/ 内でパスする

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/cli/cli.go` の `driftLines` / `buildStatusReport`、`tui/model.go` の `statusReport` / `providerFromDriftLine`

### 設計の確認
- `toolStatus` に additive な新フィールド `driftEntries`（`{"provider": ..., "op": ...}` の配列）を追加するのみ。既存フィールド・テキスト出力は触らない
- op の値は `driftLines` の文言に対応する固定集合: `not-in-tool`（`%s: ツールに無い`）/ `not-in-central`（`%s: 中央に無い`）/ `drift`（`%s: 差分あり`）。単独の `差分なし` は driftEntries に含めない（既存の `drift` と同じ除外方針）
- `schemaVersion` は 1 のまま（additive のため）
- `provsync-tui` は別モジュールのため、最小限の JSON 構造は tui 側にも定義する（現状の `statusReport` と同じ方針）

### 手順
1. テストを先に書く: `cli_test.go` に driftEntries の新規テスト、`tui/model_test.go` に優先・フォールバックのテスト。`go test ./...` で失敗することを確認する
2. `internal/cli/cli.go` の `buildStatusReport` で driftEntries を組み立て、`--json` 出力に含める（テキスト出力は不変）
3. tui 側: `driftEntries` が存在すればそれを優先し、無ければ従来の逆解析にフォールバックする
4. README の JSON スキーマ説明に driftEntries と op 値の固定集合を追記する
5. CHANGELOG `[Unreleased]` に追記する
6. `make check` を実行し、tui/ 内で `go build ./...` / `go test ./...` を実行する

### 注意
- 既存の `--json` 出力の既存フィールド・テキスト出力は 1 バイトも変えない（新フィールドの追加のみ）
- op 値の集合は固定し、後から勝手に増やさない
