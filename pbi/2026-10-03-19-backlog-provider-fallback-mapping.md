# PBI: 経路別（OpenRouter / 直接 API）の provider フォールバックマッピング

## ユーザーストーリー
OpenRouter と各社直接 API を使い分ける開発者として、ツールごとに最適な経路の provider を割り当てたい、なぜなら同じモデルでも、ツールによって OpenRouter 経由と直接 API のどちらが望ましいかが異なるから

## 優先度
- 順位: 19 / 19
- RICEスコア: 0.75（Reach=3 / Impact=1 / Confidence=50% / Effort=2）
- 根拠: 13（モデルエイリアス）に依存し、スキーマ拡張も重なる。要件の具体像（優先順か、利用可否による自動切替か）が曖昧で確信度が低い

## 設計方針（なぜなぜ分析の結果）
- 疑問: 「フォールバック」は実行時の自動切替か、同期時の割り当てか
  - provsync は設定を書き出すだけで、実行時には関与しない。実行時の自動切替は対象外
  - 解: 同期時に「ツール X には経路 A の provider、使えなければ経路 B」を決めるルールとして扱う。「使えない」の判定は環境変数の有無（`apiKeyEnv` が設定済みか）に限る。通信しない
- 疑問: ルールはどこに置くか
  - 解: 中央設定の `routes`（モデルエイリアス → 経路の優先順）とツール別の上書きで表す

## BDD受け入れシナリオ
Scenario: 直接 API を優先し、キーがなければ OpenRouter に回す
  Given `sonnet` の経路が「anthropic → openrouter」で、`ANTHROPIC_API_KEY` が未設定、`OPENROUTER_API_KEY` が設定済みである
  When  `provsync push opencode` を実行する
  Then  opencode には openrouter 経由の provider が割り当てられる

Scenario: どの経路も使えない
  Given 経路上のすべての `apiKeyEnv` が未設定である
  When  `provsync push opencode` を実行する
  Then  警告が表示され、その provider は変更されずに残る

Scenario: ツール別の上書き
  Given opencode のみ経路を openrouter 固定にしている
  When  push を実行する
  Then  opencode は openrouter、kilocode は既定の経路が割り当てられる

## 受け入れ基準
- [x] 判定は環境変数の有無のみで、通信と値の出力を行わない
- [x] 選択結果が Plan に反映され、preview / diff / apply で一致する
- [x] 13 のエイリアス機構と同じスキーマ版で導入する

## テスト戦略
- E2E: 環境変数を切り替えた push の出力
- 統合: `cli.Run --root <tmpdir>` で 3 シナリオ
- 単体: 経路選択関数（優先順、欠落、上書き）

## 見積もり
5 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 最初に確認（実装ゲート）
13（モデルエイリアス）が完了していること。要件（優先順の表現方法）が曖昧なので、着手前にユーザーへ次を確認する: 「経路の優先順は中央設定の `routes` に、エイリアス名ごとに provider キーの配列で書く形でよいか」。承認がなければ実装しない。

### 手順（承認後）
1. 先に読む: 13 で作った `internal/syncer/aliases.go`、`cmdPush`、`buildSyncPlan`、`model.Config`、`Provider.APIKeyEnv`。
2. `model.Config` に `Routes map[string][]string \`json:"routes,omitempty"\`` を足す（キー: エイリアス名、値: provider キーの優先順）。**`buildCentralChange` で `Routes: base.Routes` を引き継ぐ**（13 の `Aliases` と同じ落とし穴）。ツール別の上書きは `routesByTool map[string]map[string][]string`（`routes` と同形を tool 名で包む）に分けず、まず全ツール共通だけを実装し、ツール別は 2 段階目にする。
3. 純粋関数を `internal/syncer/routes.go` に作る。

```go
// SelectRoute は candidates の先頭から、apiKeyEnv が設定済みの provider キーを返す。
// lookup は環境変数の有無を返す(テストで差し替える)。見つからなければ ok=false。
func SelectRoute(candidates []string, providers map[string]model.Provider, lookup func(string) bool) (key string, ok bool)
```
   `apiKeyEnv` が空の provider は「環境変数不要」として候補に残す。判定は有無のみで、値を読まない。通信しない。
4. `cli`: 13 の `ResolveAliases` の前に、`routes` に載っているエイリアスについて選ばれなかった provider を `managed` から除く処理を足す（除くのは push の描画対象だけ。中央設定は変えない）。選択できなかったときは警告を `o.errOut` に出し、その provider は変更せず残す。
5. テスト: 優先 1 位のキーが未設定で 2 位が設定済みなら 2 位が選ばれる、すべて未設定なら警告で変更なし、`routes` が無ければ従来どおり、`pull` 後も `routes` が残る。環境変数は `t.Setenv` で制御。

### 注意
- 実行時の自動切替は対象外（provsync は設定を書くだけ）。
- 判定のために HTTP 通信をしない（疎通確認は 18 の `check`）。
