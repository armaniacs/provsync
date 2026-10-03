# PBI: 実装ガイドと AGENTS.md のコードの地図を現状に同期する

## 種別
- docs（コード変更なし。ドキュメントの構造ドリフト修正）

## ユーザーストーリー
provsync の新規貢献者として、実装ガイド（00-implementation-guide.md）のコードの地図が cli 責務分割前の姿のままであるため、地図を現状に同期したい、なぜならガイドは「各 PBI の実装ガイドが前提にする」SSOT 文書であり、ドリフトした手順（§5 の「cli.go の switch に case を足す」）に従うと現行の commandRegistry ベースの dispatch に沿わない変更を入れてしまうから

## 優先度
- 順位: 1
- RICEスコア: 7.0（Reach=7 / Impact=1 / Confidence=1.0 / Effort=1）
- 依存: なし
- 根拠: 台帳 `2026-10-03-00-backlog-holisticb.md` の候補 C3（Quick Win）

## 背景（レビューで確認済みの実在箇所。行番号は概略）
- `pbi/00-implementation-guide.md` の「3. コードの地図」（概略 28-46 行）:
  - `internal/cli/cli.go` の行（概略 33 行）が `Run`, `options`, `registerFlags`, `reorder`, `usage`, `cmdList/Status/Pull/Push/Sync/Diff/Undo`, `applyOrPreview`, `buildCentralChange`, `buildToolChange`, `buildSyncPlan` を 1 ファイルの関数一覧として記載しているが、現行は cli 責務分割後の構成で、`usage` とコマンドレジストリは `internal/cli/help.go`、`cmdList`/`cmdStatus` は `status.go`、`cmdInit`/`cmdPull`/`cmdPush`/`cmdSync`/`buildCentralChange`/`buildToolChange`/`buildSyncPlan` は `sync.go`、`cmdDiff`/`cmdUndo` は `history.go`、`cmdDoctor` は `doctor.go`、`cmdCheck` は `check.go` に分散している
  - 地図に `internal/lock`（flock 排他）、`internal/secret`（秘密キー判定・マスク）、`internal/version`（バージョン解決）、`tui/`（bubbletea TUI・独立モジュール）が載っていない
- `pbi/00-implementation-guide.md` の「5. 新しいサブコマンドを足す手順」（概略 60-67 行）が「`internal/cli/cli.go` の `Run` の `switch cmd` に `case` を足す」と記載しているが、現行は `internal/cli/help.go` の `commandRegistry()`（概略 31 行）に 1 エントリ追加する形になっている
- ルートの `AGENTS.md` の Layout セクションが `internal/lock`、`internal/secret`、`internal/version` の記載を欠いている（`internal/cli/` の説明も分割後の構成と合っていない）

## 改善案
1. `pbi/00-implementation-guide.md` のコードの地図を現行構成に合わせる:
   - `internal/cli` の行を分割後のファイル構成（cli.go / sync.go / status.go / history.go / help.go / doctor.go / check.go / errors.go / render.go / platform.go）に更新する
   - `internal/lock`、`internal/secret`、`internal/version`、`tui/` の行を追加する
2. §5「新しいサブコマンドを足す手順」を commandRegistry ベースの手順に書き換える（`internal/cli/help.go` の `commandRegistry()` に 1 エントリ追加、文言は internal/i18n のカタログに追加）
3. `AGENTS.md` の Layout セクションに `internal/lock`、`internal/secret`、`internal/version` を追記し、`internal/cli/` の説明を分割後の構成に合わせる
4. ドキュメントの更新のみ。コード・テストは一切変更しない

## 制約
- コード・テスト・CI 設定を変更しない
- ドキュメントの既存の規約（日本語、絵文字なし、issue 参照・経緯は書かず最新仕様のスナップショットのみ）を守る
- 地図の記載は実在する関数・型・パスのみに限る（推測で書かない）

## BDD受け入れシナリオ
Scenario: 新規貢献者がガイドに従ってコマンドを追加する
  Given 実装ガイドが現行構成に同期されている
  When  新規サブコマンドを追加する手順を読む
  Then  手順は commandRegistry ベースの現行 dispatch と一致している

Scenario: 新規貢献者が lock パッケージの役割を地図で見つける
  Given コードの地図に internal/lock が載っている
  When  排他ロックの実装を探す
  Then  地図から internal/lock/lock.go にたどり着ける

## 受け入れ基準
1. `pbi/00-implementation-guide.md` のコードの地図が現行のファイル構成と一致する（cli の分割後構成、lock/secret/version/tui の追加）
2. §5 の手順が commandRegistry ベースになっている
3. `AGENTS.md` の Layout セクションに lock/secret/version が載っている
4. 地図に載せた関数・型・パスがすべて実在する（grep で確認する）
5. コード・テストに差分がない（`git diff` が docs のみ）
6. `make check` がパスする（コードを触らないため変化しないが、完了前に実行する）

## テスト戦略
- テスト対象コードはない。地図の各記載（ファイル・関数・型名）を grep/ls で実在確認する
- `make check` を実行して既存テストが壊れていないことを確認する

## 見積もり
1 SP

## Definition of Done
- [ ] 実装ガイドのコードの地図が現行構成と一致する
- [ ] §5 が commandRegistry ベースの手順になっている
- [ ] AGENTS.md の Layout に lock/secret/version が載っている
- [ ] 地図の記載がすべて実在する（grep 確認済み）
- [ ] コードへの差分がない
- [ ] `make check` がパスする

## 実装ガイド（この順に実施。先に /Users/yaar/Playground/provsync/pbi/00-implementation-guide.md を読む）

### 先に読むファイル
`pbi/00-implementation-guide.md` 全体、`AGENTS.md`、`internal/cli/` のファイル一覧、`internal/lock/lock.go`、`internal/secret/secret.go`、`internal/version/`、`tui/go.mod`。

### 手順
1. `ls internal/cli/` と `ls internal/` で現行構成を確認する
2. 実装ガイドのコードの地図を現行構成に更新する（関数・型名は grep で実在確認してから書く）
3. §5 を commandRegistry ベースの手順に書き換える
4. AGENTS.md の Layout セクションを更新する
5. `make check` を実行する

### 注意
- コメント・ドキュメントは日本語、絵文字なし。issue 参照・変更履歴は書かず、最新仕様のスナップショットのみを書く
- 担当ファイルは `pbi/00-implementation-guide.md` と `AGENTS.md` の 2 つのみ。他のファイルには触れない
- README.md は本 PBI の対象外（レビューでドリフトは検出されていない）
