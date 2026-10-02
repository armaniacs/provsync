# PBI: pull/push 前処理パイプラインの重複を共通ヘルパーに集約する

## ユーザーストーリー
provsync の保守担当者として、pull/push の前処理パイプラインが 3〜4 箇所に重複しているため、共通ヘルパーに抽出して新ステップ追加時の波及を 1 箇所にしたい、なぜなら重複した系列をそれぞれ個別に直すと直し漏れが起きやすく、小さな変更でも複数箇所の同期を確認する負担が続くから

## 優先度
- 順位: 2
- RICEスコア: 1.67（Reach=5 / Impact=1 / Confidence=1.0 / Effort=3）
- 種別: refactor。挙動不変（外部出力・終了コード・テスト結果は全く同じ）・最小差分
- 依存: PBI 20（cli.go 責務分割）の後に実施する
- 出所: 台帳 `2026-10-03-00-backlog-holistic.md` の候補 C2

## 背景
レビューで確認された実在の重複。行番号は概略であり、PBI 20 実施後は `internal/cli/cli.go` が分割されるため、実装時に現位置を確認すること。

- pull 側の同一系列「`adapter.Get` → `Pull` → 警告を stderr へ → `syncer.FilterProviders` → `buildCentralChange`」が 3 箇所に重複している:
  - `cmdPull`（旧 `internal/cli/cli.go` 621-632 付近）
  - `cmdInit`（530-545 付近）
  - `buildSyncPlan` の from 分岐（782-794 付近）
- push 側の同一系列「`syncer.FilterProviders` → `o.applyRoutes` → `o.applyAliases` → `buildToolChange`」が 2 箇所に重複している:
  - `cmdPush`（694-705 付近）
  - `buildSyncPlan` の to 分岐（810-821 付近）
- 付随して、`backup.New(root.StateDir())` + `keepFromEnv` + `SetMax` のセットが `cmdUndo` と `applyOrPreview`（バックアップ記録部）に重複している

## BDD受け入れシナリオ
Scenario: pull / init / sync --from の前処理は 1 箇所で直せる
  Given pull、init、sync --from の各経路が pull 側前処理系列を共有している
  When 新しい前処理ステップを追加する
  Then  抽出されたヘルパー（`pullFromTool` 相当）を 1 箇所だけ直せば 3 経路すべてに反映される

Scenario: push / sync --to の前処理は 1 箇所で直せる
  Given push と sync --to の各経路が push 側前処理系列を共有している
  When push 前処理を追加する
  Then  抽出されたヘルパー（`buildManagedForPush` 相当）を 1 箇所だけ直せば 2 経路すべてに反映される

Scenario: 抽出前後で出力が一切変わらない
  Given 抽出前の `pull` / `push` / `sync` / `init` のプレビュー出力・`--write` 実行結果・stderr の警告を記録してある
  When  前処理系列を共通ヘルパーへ抽出する
  Then  すべてのコマンド出力・終了コード・テスト結果が抽出前と全く同じである

## 受け入れ基準
- [x] 既存テスト（`internal/cli/cli_test.go` 全体）を 1 行も変更せずに全テストがパスする
- [x] `make check` がパスする
- [x] pull 側前処理系列の重複（3 箇所）が 1 箇所のヘルパーに集約されている
- [x] push 側前処理系列の重複（2 箇所）が 1 箇所のヘルパーに集約されている
- [x] 警告の出力先（stderr）・文言・順序が不変である
- [x] `pull` / `push` / `sync` / `init` のプレビュー出力が抽出前と byte-identical である

## テスト戦略
- 移動前に現行の出力（`pull` / `push` / `sync` / `init` のプレビューと `--write` 実行結果、stderr の警告）を記録して pin する
- 抽出後に byte-identical を確認する
- 既存テスト（`internal/cli/cli_test.go` 全体）が parity を保証する。テストは原則変更しない

## 見積もり
3 SP

## Definition of Done
- [x] 既存テスト（`internal/cli/cli_test.go` 全体）を 1 行も変更せずに全テストがパスする
- [x] `make check` がパスする
- [x] pull 側前処理系列の重複（3 箇所）が 1 箇所のヘルパーに集約されている
- [x] push 側前処理系列の重複（2 箇所）が 1 箇所のヘルパーに集約されている
- [x] 警告の出力先（stderr）・文言・順序が不変である
- [x] `pull` / `push` / `sync` / `init` のプレビュー出力が抽出前と byte-identical である

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
- [00-implementation-guide.md](00-implementation-guide.md)
- `internal/cli/cli.go` の `cmdPull` / `cmdInit` / `cmdPush` / `buildSyncPlan` / `applyOrPreview` / `cmdUndo` / `buildCentralChange` / `buildToolChange`
- 台帳 `2026-10-03-00-backlog-holistic.md` の候補 C2（依存先は PBI 20）

### 手順
1. 抽出前に現行出力を記録して pin する: `pull` / `push` / `sync` / `init` のプレビュー出力、`--write` 実行結果、stderr の警告。記録はリポジトリ外（一時ディレクトリ）に置く。
2. pull 側系列「`adapter.Get` → `Pull` → 警告を stderr へ → `syncer.FilterProviders` → `buildCentralChange`」を 1 つのヘルパーに抽出する（名前の例: `pullFromTool`。最終名は実装者が決めてよい）。`cmdPull` / `cmdInit` / `buildSyncPlan` の from 分岐の 3 呼び出し箇所を置換する。
3. push 側系列「`syncer.FilterProviders` → `o.applyRoutes` → `o.applyAliases` → `buildToolChange`」を 1 つのヘルパーに抽出する（名前の例: `buildManagedForPush`）。`cmdPush` / `buildSyncPlan` の to 分岐の 2 呼び出し箇所を置換する。
4. `backup.New(root.StateDir())` + `keepFromEnv` + `SetMax` のセットを `newBackupStore(root)` のようなヘルパーに集約してよい。`cmdUndo` と `applyOrPreview`（バックアップ記録部）を置換する。
5. 警告出力（stderr）の責務は抽出後も同じ位置に維持する。エラーハンドリングは現状のまま変えない。
6. `make check` を実行して全部通ることを確認する。手順 1 の記録と diff し、byte-identical を確認する。

### 注意
- この PBI はリファクタリングであり、挙動を一切変えない。警告の文言・出力先・順序も不変とする。外部出力・終了コード・テスト結果は全く同じであること。
- 既存テスト（`internal/cli/cli_test.go` 全体）は原則変更しない。テストを書き換えないと通らない場合は、実装ガイド §7 に従って実装を止めて報告する。
- 挙動不変のため「テストを先に書く」型（実装ガイド §1 の手順 3〜4）は本 PBI には適用しない。parity は既存テストと手順 1 の出力記録で保証する。
- 背景の行番号は概略。PBI 20（cli.go 責務分割）実施後はファイルが分割されているため、実装時に現位置を確認すること。
- ユーザー可視の変更がないため README / CHANGELOG の更新は不要とする。
