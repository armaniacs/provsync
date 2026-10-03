# PBI: CLI メッセージを含む全ユーザー向け出力の i18n 化（ja/en）

## ユーザーストーリー
日本語を読めない provsync の利用者・貢献者として、ヘルプ・エラー・警告・進行状況など CLI のメッセージとルート README を自分の言語（英語）で読みたい、なぜならドキュメントサイトは mkdocs-static-i18n で既に日英対応なのに CLI の出力だけ日本語固定で、非日本語ユーザーがエラー内容や使い方を理解できず、採用の障壁になっているから

## 優先度
- 順位: 単独 PBI（バッチ外）
- RICEスコア: 4.3（Reach=8 / Impact=2 / Confidence=80% / Effort=3）
- 根拠: ドキュメントサイト（docs/*.en.md、ja 既定 / en 切替）は対応済みで、残りのギャップは CLI 出力とルート README のみ。既存ユーザーの既定動作（日本語）を変えない追加実装のため impact は中程度。一方、対象文字列がパッケージ全体に広がるため effort は大。完了後は第 3 言語の追加コストが「カタログ 1 ファイル + 完全性テスト」まで下がる

## 設計方針
- メッセージカタログは新規パッケージ `internal/i18n` に置く。`map[string]string`（メッセージ ID → テンプレート）を ja / en の 2 言語分持ち、ja を正とする。`golang.org/x/text` や go-i18n は使わない（`go.mod` は標準ライブラリのみの規則を維持）
- 言語判定は `PROVSYNC_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > 既定 `ja` の優先順位。値は前方一致（`ja*` → ja、`en*` → en）。未対応ロケール・空値はエラーにせず ja にフォールバックする
- 既定動作は変えない。言語系の環境変数を設定しない限り、現在と同じ日本語で出力される
- 対象は実行時のユーザー向け出力のみ（ヘルプ本文、エラー、警告、進行状況、使い方メッセージ）。コードコメントとドキュメント本文は対象外
- 機械可読出力（`--json`）はキー名と構造を変えないが、値の一部（`status --json` の `drift` ラベル・`warnings`）はロケール連動で翻訳される（深掘りでユーザー決定）。仕様は README / docs に明記する
- README は単一バイリンガル構成（`## 日本語` / `## English`）のまま維持する（深掘りでユーザー決定）。README.en.md は作らない。README / docs 内の「CLI メッセージは日本語固定」の注意書きのみ新仕様に更新する
- tui は独立 Go モジュール（tui/go.mod、bubbletea 依存）のため本次対象外とし、別 PBI に切り出す（深掘りでユーザー決定）

## BDD受け入れシナリオ

Scenario: 英語ロケールのユーザーが英語でメッセージを読める
  Given PROVSYNC_LANG=en の環境で kilocode / opencode の設定がある
  When  `provsync status` を実行する
  Then  出力・警告・エラーがすべて英語で表示される
  And   中央設定・ツール設定ファイルは一切変更されない

Scenario: 既定では現在と同じ日本語で表示される
  Given 言語に関わる環境変数を設定しない環境
  When  `provsync status` を実行する
  Then  メッセージは従来どおり日本語で表示される

Scenario: 未対応ロケールはエラーにならず日本語で動く
  Given PROVSYNC_LANG=fr の環境
  When  `provsync list` を実行する
  Then  メッセージは日本語で表示され、終了コードは 0

Scenario: 機械可読出力の構造は言語の影響を受けない
  Given PROVSYNC_LANG=en の環境
  When  `provsync status --json` を実行する
  Then  JSON のキーと値の構造は日本語実行時と同一である
  And   drift ラベルと warnings の値は英語で表示される（ロケール連動の仕様）

Scenario: README と docs の注意書きが新仕様と一致する
  Given README.md と docs/usage.md・docs/usage.en.md に CLI メッセージに関する注意書きがある
  When  i18n 化が完了する
  Then  「CLI メッセージは日本語固定」の記述がなく、新仕様（ja 既定・PROVSYNC_LANG / LANG で en）が書かれている

## 受け入れ基準
- [x] internal/cli / internal/adapter / internal/backup のユーザー向け出力文字列（ヘルプ・エラー・警告・進行状況・使い方）がすべて `internal/i18n` のカタログ経由になる
- [x] カタログ完全性テスト: 全メッセージ ID が ja / en 両方に存在することを自動テストで保証する
- [x] 言語判定の優先順位とフォールバック（`ja*` / `en*` / 未対応 / 空値 / 未設定）を単体テストで検証する
- [x] 環境変数を設定しない実行の日本語出力が移行前と同等であることを統合テストで検証する
- [x] `--json` など機械可読出力の構造が言語に依存しないことをテストで検証する
- [x] `internal/i18n` 以外にユーザー向け日本語出力が残らない（`fmt.Errorf` / `fmt.Fprintf` / `fmt.Printf` / `fmt.Println` / `fmt.Sprintf` の呼び出し引数を rg で検証し 0 件）
- [x] 秘密の値がメッセージに出ない既存規則は維持される
- [x] `--json` の drift ラベル・warnings 値がロケールに連動し、その仕様が README / docs に明記されている
- [x] README.md・docs/usage.md・docs/usage.en.md の「CLI メッセージは日本語固定」の注意書きが新仕様に更新されている
- [x] tui の i18n が別 PBI（backlog）に切り出されている
- [x] AGENTS.md と pbi/00-implementation-guide.md §2 の「ユーザー向けメッセージは日本語」規則を新仕様（カタログに ja / en 両方を追加、ja が正）に更新する
- [x] `go.mod` に依存を追加していない（`git diff go.mod` が空）
- [x] `make check` がパスする

## テスト戦略
- E2E（cli.Run レベル）: `setup` / `mustRun` 補助関数で、`t.Setenv("PROVSYNC_LANG", "en")` を付けた全サブコマンド（list / status / pull / push / sync / diff / undo / doctor）の英語出力と、無設定時の日本語出力を検証する
- 統合: `UsageError` 経由の使い方メッセージが ja / en 両ロケールで期待どおりに出ること。`--json` の ja / en 構造同一性
- 単体: カタログ完全性（全 ID × 全ロケールの存在、書式動詞の種類一致）、ロケール判定の境界値（`ja` / `ja_JP.UTF-8` / `en` / `en_US` / `fr` / 空文字 / 未設定）、`T()` の補間（`%s` / `%d` を含むテンプレート）、JSON drift / warnings のロケール連動（en で英語・ja で日本語・未対応ロケールで ja）

## 見積もり
6 ポイント（要チームでの見積もり。tui と README 分離を除外、JSON ロケール連動を含む。深掘り後に縮小）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` / `make test-race` がパスする
- [ ] コードレビュー完了（GitHub PR での approve を必須とする。ローカルでのレビューエージェントによるレビューと指摘反映は完了、PR 作成・承認は別途）
- [x] リファクタリング完了（グリーン後）
- [x] ロールバック手段: 単純 revert で可。データ移行がなく、無設定時の既定動作を変えないため後方互換（技術的考慮事項に記載）
- [x] ドキュメント更新済み（README.md・docs/usage.md・docs/usage.en.md、AGENTS.md、pbi/00-implementation-guide.md、CHANGELOG。README は単一バイリンガル維持のため README.en.md は作らない）

## 技術的考慮事項
- 依存関係: なし。`go.mod` への追加は禁止（標準ライブラリのみの規則を維持）
- テスタビリティ: `internal/i18n` の判定は純粋関数（環境変数の値は引数で受け、`os.Getenv` は `cli.Run` から 1 回だけ呼ぶ）。cli テストは既存補助関数 + `t.Setenv`
- 非機能要件: ロケール解決は起動時に 1 回のみ。英語メッセージは日本語より長くなるため表の桁ずれを統合テストで確認
- ロールバック: revert で可。カタログ追加は後方互換で、データ移行・既定動作の変更なし

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/cli/cli.go`（`Run` / `options` / `usage` / `reorder`）、`internal/cli/help.go`（ヘルプ本文）、`internal/cli/errors.go`（`UsageError`）、`internal/cli/status.go`、`internal/cli/sync.go`、`internal/adapter/adapter.go`、`internal/backup/backup.go`、`tui/`、README.md、docs/cli.md と docs/cli.en.md、mkdocs.yml（`i18n` プラグイン設定）。

### 事実の整理（2026-10-03 作成時に確認済み）
- Go コードに i18n 機構は存在しない（`rg -i "i18n|locale|localize|translation|message.?catalog" --type go` で確認済み）。CLI メッセージは日本語直書き。
- ドキュメントサイトは mkdocs-static-i18n（`docs_structure: suffix`、ja 既定 / en 切替）で日英対応済み。docs/*.en.md が存在する。ルート README.md だけ英語版が無い。
- ユーザー向け出力の所在: internal/cli/help.go（最大）、sync.go、cli.go、status.go、doctor.go、history.go、check.go、render.go、errors.go、internal/adapter/adapter.go、internal/backup/backup.go、tui/。`fmt.Errorf` は全体で 32 件。
- サブコマンド: list / status / pull / push / sync / diff / undo / doctor（internal/cli/cli.go の switch で確認）。
- メッセージは `fmt.Fprintf(o.out, "中央設定: %s (%d providers)\n", ...)` のように printf 形式の引数を持つ。カタログは Go の書式動詞を保持する必要がある。
- README.md は単一バイリンガル構成（`## 日本語` / `## English` が镜像、770 行、バッジなし、行 6 にアンカーセレクタ）。README.en.md は存在しない。docs/index.md は README とは別内容の手動管理ページ。
- tui は独立 Go モジュール（tui/go.mod、bubbletea + x/term、provsync-tui バイナリ）。親モジュールの `internal/i18n` は import できない。
- `status --json` の `drift` / `warnings` には今日も日本語が載る（`driftOpLabel`、adapter 警告）。
- internal パッケージ（store / lock / fsutil / syncer / backup / adapter）の `fmt.Errorf` は main.go が stderr に出すため全てユーザー向け（約 20 か所）。
- バックアップ manifest に保存されるラベル（`pull kilocode` 等）と doctor の Status 判別子（`OK` / `警告` / `NG` の switch）は翻訳不可・要分離。
- 補完スクリプト（`cmdCompletion`）は機械可読のため翻訳しない。

### 手順
1. `internal/i18n` パッケージを新設する。判定と取得は純粋関数にする:

```go
// Package i18n はユーザー向けメッセージの言語判定とカタログ参照を提供する。
package i18n

// Resolve は環境変数の値から ja / en を返す。未対応・空は ja にフォールバックする。
func Resolve(lang string) string

// T はカタログからテンプレートを取得し fmt.Sprintf で補間する。
func T(lang, id string, a ...any) string
```

   `Resolve` の優先順位は `PROVSYNC_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > ja。前方一致で `ja*` → ja、`en*` → en、それ以外・空は ja。
2. カタログを `internal/i18n/ja.go` と `internal/i18n/en.go` に `map[string]string`（ID → テンプレート）で持つ。ja が正。en に無い ID があると完全性テストが失敗する設計にする。
3. `internal/i18n/i18n_test.go` に完全性テスト（全 ID × ja/en 存在、書式動詞の種類一致）とロケール判定テスト（`ja` / `ja_JP.UTF-8` / `en` / `en_US` / `fr` / `""` / 未設定）を書く。先に失敗することを確認する。
4. `cli.Run` 冒頭でロケールを解決し `options` に持たせる。
5. パッケージ順に移行する（1 パッケージ = 1 コミット目安）: help.go → sync.go → cli.go（usage・フラグ説明含む）→ status.go / doctor.go / history.go / check.go / render.go → errors.go（使い方メッセージ）→ adapter.go / backup.go → store / syncer / lock / fsutil / version など実行時出力を持つ残りのパッケージ（tui は対象外）。各パッケージで既存の日本語出力テストを `t.Setenv` による言語明示に切り替える。
6. 全サブコマンドの ja / en 統合テストを cli_test.go に追加する（`setup` / `mustRun` 使用）。
7. tui の i18n を別 PBI（`pbi/2026-10-03-25-backlog-tui-i18n.md`）として切り出す。
8. README.md・docs/usage.md・docs/usage.en.md の「CLI メッセージは日本語固定」の注意書き（README.md の English セクション内 Note、usage.md 冒頭の `!!! note`）を新仕様に更新する。README の構造・セレクタは変更しない。
9. AGENTS.md と pbi/00-implementation-guide.md §2 の「ユーザー向けメッセージ・コメント・ドキュメントは日本語」を新仕様に書き換える（コードコメントは日本語のまま、ユーザー向けメッセージはカタログに ja / en 両方を追加し ja が正）。
10. CHANGELOG.md の `[Unreleased]` に追記する。README / docs（cli.md・cli.en.md）に出力例があれば現行仕様に合わせる。

### 注意
- 対象は実行時出力のみ。コードコメントは日本語のまま維持し、カタログに掃き込まない。
- cli_test.go の期待値は日本語文字列で直書きされている。既存テストの変更は「言語明示化」の範囲にとどめ、検証内容は変えない（仕様変更にしない）。
- ヘルプ本文（help.go）は複数行の大きなテキスト。カタログ ID の切り方（コマンド単位・節単位）を移行前に決め、途中で変えない。
- `%-9s` などの整形動詞はテンプレート側に保持する。英語の方が長く表の桁がずれないか統合テストで確認する。
- README は構造を変えない（アンカーセレクタ・镜像構造維持）。注意書きの文言のみ更新する。
- 既定（無設定時）を ja から変えない。en 既定にするかどうかは別 PBI での判断。
- docs サイト（mkdocs.yml の nav_translations）には影響しない。docs/cli.en.md の出力例がある場合のみ現行仕様に合わせる。
- 秘密の値がメッセージに出ない規則は変更しない。

## 深掘りセッション — 2026-10-03（feature-dev フロー・ユーザー回答あり）

### 挑戦した仮定
| 仮定 | リスク | 発見 | 決定 |
|------|--------|------|------|
| `--json` は言語の影響を受けない | 高 | `status --json` の drift / warnings には今日も日本語が載る。ローカライズしないと en でも日本語が残り、すると JSON 値がロケール依存になる | JSON もロケール連動で翻訳（ユーザー決定）。構造（キー・型）は不変、仕様を README / docs に明記 |
| 「全て」に tui が含まれる | 高 | tui は独立 Go モジュール（bubbletea 依存）で `internal/i18n` を import できない。drift 逆解析フォールバックも ja 依存 | tui は別 PBI に切り出し（ユーザー決定）。本次は CLI + 規約ドキュメントのみ |
| README.en.md を新規作成する | 中〜高 | README.md は既に単一バイリンガル（镜像・セレクタ済み・バッジなし）。分離は作り直しになる | README は単一バイリンガル維持（ユーザー決定）。注意書きの更新のみ |
| internal パッケージのエラーはスコープ外 | 中 | main.go が全エラーを stderr に出すため store / lock / fsutil / syncer / backup / adapter の約 20 か所もユーザー向け | 移行対象に含める |
| doctor の Status は表示専用 | 中 | 「OK / 警告 / NG」リテラルが switch 判別子としても機能する | 内部キー（機械値）と表示を分離し、表示のみカタログ経由 |

### 新たに発見したリスク
- tui が子プロセスで `provsync status --json` を実行するため、tui 端末の LANG が en だと drift 行が英語になる（tui 側は DriftEntries 優先で緩和済み。別 PBI で確認）
- 言語判定がプロセス env に依存するため、CI の LANG 差異で既存テストが言語を変えて失敗しうる → `setup(t)` で PROVSYNC_LANG を pin する
- フラグ説明 15 件（cli.go registerFlags）が flag パッケージ経由の隠れた出力面
- バックアップ manifest 保存ラベル（`pull kilocode` 等）は翻訳すると undo --list とテストが壊れる → 翻訳対象外

### 未解決の疑問
- なし（残りは実装詳細として feature-dev フロー内で判断する）

### 決定事項
1. JSON 値（drift / warnings）はロケール連動で翻訳。構造は不変
2. tui は別 PBI（2026-10-03-25-backlog-tui-i18n）に切り出し。本次対象外
3. README は単一バイリンガル維持。README.en.md は作らない。注意書き更新のみ
4. internal パッケージのユーザー向けエラーもカタログ対象
5. 見積もりを 8 → 6 ポイントに縮小
