# PBI: モデル名の共通エイリアスとツール別 ID への正規化

## ユーザーストーリー
複数の LLM ツールを使う開発者として、`sonnet` のような共通名で書いたモデルを各ツールが要求する ID 形式へ自動変換してほしい、なぜならツールごとに `claude-3-5-sonnet` / `anthropic/claude-3-5-sonnet` / `claude-3.5-sonnet` と綴りが違い、手で書き分けるのは間違いのもとだから

## 優先度
- 順位: 13 / 19
- RICEスコア: 3（Reach=6 / Impact=2 / Confidence=50% / Effort=2）
- 根拠: 価値は大きいが、モデル ID の仕様は各社・各ツールで頻繁に変わり確信度が低い。現状の `models` は ID をキーに持つ不透明なマップで、スキーマ拡張を伴う。ユーザーが明確な不便を感じるまで、導入体験・安全性・配布の PBI を先に行う

## 設計方針（なぜなぜ分析の結果）
- 疑問: 共通名の対応表を誰が保守するのか
  - 組み込み表はモデル更新のたびにリリースが要り、陳腐化する
  - 解: 対応表は中央設定内の `aliases`（ユーザー定義）を正とし、組み込みはごく少数の初期例にとどめる。表にない名前は変換せず素通しする
- 疑問: 変換の失敗はどう扱うか
  - 解: 未解決のエイリアスは警告して素通し、`--strict` でエラーにする。秘密と同様、推測で書き換えない
- 疑問: 逆方向（pull）はどうするか
  - 解: pull は取り込んだ ID をそのまま保持し、エイリアスへの逆変換は行わない。往復で情報を失わない既存方針を守る

## BDD受け入れシナリオ
Scenario: 共通名がツール別の ID に変換される
  Given 中央設定に `sonnet` のエイリアスがあり、kilocode 用と opencode 用の ID が定義されている
  When  `provsync push opencode --write` を実行する
  Then  opencode の設定には opencode 用の ID で書き込まれ、kilocode には影響しない

Scenario: 当該ツール向けの対応がないエイリアスは素通しされ警告される
  Given エイリアス `sonnet` に opencode 向けの ID が定義されていない
  When  `provsync push opencode` を実行する
  Then  名前はそのまま出力され、「エイリアス未定義」の警告が表示される

Scenario: strict では未定義がエラーになる
  Given エイリアス表にないモデル名がある
  When  `provsync push opencode --strict --write` を実行する
  Then  何も書き込まれずエラーで終了する

Scenario: pull は ID を書き換えない
  Given ツール設定に `anthropic/claude-3-5-sonnet` がある
  When  `provsync pull opencode --write` を実行する
  Then  中央設定には元の ID がそのまま保存される

## 受け入れ基準
- [x] 中央設定に `aliases` を追加し、`model.Version` を上げて旧版を読める（前方互換の方針を決める）
- [x] 変換は adapter 層で行い、`internal/syncer` の merge ロジックには持ち込まない
- [x] Plan に変換後の内容が反映され、preview・diff・apply・backup が同じ結果を使う
- [x] 標準ライブラリのみ。組み込みの対応表はデータとして分離する
- [x] README に定義例と未定義時の挙動を記載

## テスト戦略
- E2E: 実ファイルで push の出力が期待 ID になること
- 統合: `cli.Run --root <tmpdir>` で 4 シナリオ、pull→push の往復で不変
- 単体: エイリアス解決（定義あり・なし・循環）、スキーマ版の読み込み

## 見積もり
5 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/model/model.go`、`internal/syncer/syncer.go`、`internal/store/store.go`、`cli.go` の `cmdPush` と `buildSyncPlan`、`plan.go`。

### 実装上の確定事項（PBI の曖昧さを解消）
- 中央設定にトップレベル `aliases` を足す。形は `{"sonnet": {"kilocode": "<kilo 用 ID>", "opencode": "<opencode 用 ID>"}}`（エイリアス → ツール名 → モデル ID）。
- スキーマは追加のみ（`omitempty`）なので `model.Version` は 1 のまま変えない。古い provsync は `aliases` を無視して読める。
- モデルのキー（`Provider.Models` のマップのキー）が `aliases` のキーと一致するときだけ変換する。一致しないキーは通常のモデル ID としてそのまま通し、警告も出さない。
- 一致したが、そのツール向けの ID が定義されていないとき: 警告を出し、キーはそのまま通す。`--strict` 指定時はエラーにして何も書かない。
- 変換は push の描画前にだけ行う。pull は ID を変えない。

### 手順
1. `model.Config` に `Aliases map[string]map[string]string \`json:"aliases,omitempty"\`` を足す。`store.Marshal` / `Load` はそのまま使える（テストで往復を確認）。
2. `internal/syncer/aliases.go` に純粋関数を作る（外部状態に触れない）:

```go
// ResolveAliases は managed の各 provider の Models のキーを、tool 向けの ID へ置き換えた
// コピーを返す。未定義の対応は warnings に積み、キーはそのまま残す。
func ResolveAliases(managed map[string]model.Provider, aliases map[string]map[string]string, tool string) (map[string]model.Provider, []string)
```
   - 入力の `managed` と `Models` を破壊しない（新しいマップへコピーする）。
   - モデルの値（`Models[key]` の中身）はそのままコピーする。
3. `internal/syncer/aliases_test.go`: 変換される、一致しないキーは不変、ツール向け未定義で警告、入力が不変、`aliases` が nil なら何もしない。
4. `cli.go`: `cmdPush` と `buildSyncPlan` で、`FilterProviders` の直後・`buildToolChange` の前に `managed, warns = syncer.ResolveAliases(managed, central.Aliases, a.Name())` を呼び、警告を `o.errOut` に出す。`--strict`（`options.strict`）で `warns` が 1 件以上ならエラーを返して終了する。`buildCentralChange`（pull 側）には何も足さない。
5. `buildCentralChange` は `model.Config{Version, Providers}` を新しく作るので、既存の `aliases` が pull で消えないよう `Aliases: base.Aliases` を引き継ぐ。**これを忘れると pull のたびに `aliases` が消える。** テストで確認する。
6. テスト（`cli_test.go`）: `aliases` を手で書いた中央設定を `write` で作り、`push opencode --write` で opencode 用 ID に変換される、kilocode には影響しない、`--strict` で未定義がエラー、`pull` 後も `aliases` が残る、pull は ID を書き換えない。
7. README に `aliases` の書式と挙動を追記。

### 注意
- 組み込みの既定エイリアス表は作らない（保守できないため）。ユーザー定義のみ。
- `Plan` に変換後の内容が入るので、preview / diff / apply は自動的に一致する。`cli` で別計算しない。
