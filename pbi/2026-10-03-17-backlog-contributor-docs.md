# PBI: 貢献者向けドキュメント（アダプタ追加ガイド・CONTRIBUTING）

## ユーザーストーリー
貢献者として、新しいツールのアダプタを追加する手順と報告窓口を知りたい、なぜなら provsync の価値は対応ツールの広がりにあり、追加の障壁が高いと広がらないから

## 優先度
- 順位: 17 / 19
- RICEスコア: 1.6（Reach=2 / Impact=0.5 / Confidence=80% / Effort=0.5）
- 根拠: 現状の利用者は少なく、貢献者はさらに少ない。公開・配布（09）の後に整えれば足りる。脆弱性の報告窓口（SECURITY.md）は 03 に分離して前倒し済み

## BDD受け入れシナリオ
Scenario: アダプタ追加の手順に従って新ツールを足せる
  Given CONTRIBUTING のアダプタ追加ガイドがある
  When  貢献者が手順どおりにアダプタを実装する
  Then  共通の契約テストがそのアダプタに対して実行され、満たすべき振る舞いが分かる

## 受け入れ基準
- [x] アダプタの契約（`Adapter` IF、Extras の保存と復元、秘密の扱い）をガイドに記述
- [x] 全アダプタに適用する共通の契約テスト（往復冪等、Extras 保持、秘密キー保持）を `internal/adapter` に追加
- [x] `CONTRIBUTING.md`（`make check`、コミット規約、CHANGELOG 更新）
- [x] Issue / PR テンプレート（任意）

## テスト戦略
- E2E: 対象外
- 統合: 契約テストを既存 2 アダプタで通す
- 単体: 対象外

## 見積もり
2 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 契約テストが自動実行されパスする
- [x] `make check` がパスする
- [x] ドキュメントは最新仕様のスナップショットのみ（経緯を書かない）

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/adapter/adapter.go` の `Adapter` インターフェイスと `carryOverSecrets`、`kilocode.go`、`opencode.go`、`adapter_test.go`、`AGENTS.md`。

### 手順
1. 契約テスト `internal/adapter/contract_test.go` を書く。`Names()` の全ツールに同じテストを回す（ツールを足すと自動で検査される）。

```go
func TestAdapterContract(t *testing.T) {
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			a, err := Get(name, Root{ConfigHome: dir, StateHome: dir})
			if err != nil { t.Fatal(err) }
			if err := os.MkdirAll(filepath.Dir(a.Path()), 0o755); err != nil { t.Fatal(err) }
			seed := `{"provider":{"p1":{"name":"P1","npm":"pkg","options":{"baseURL":"https://a/v1","apiKey":"sk-test-KEEP"},"models":{"m":{"name":"m"}},"custom":true}}}`
			if err := os.WriteFile(a.Path(), []byte(seed), 0o644); err != nil { t.Fatal(err) }

			providers, _, err := a.Pull()
			if err != nil { t.Fatal(err) }
			out1, err := a.Push(providers)
			if err != nil { t.Fatal(err) }
			os.WriteFile(a.Path(), out1, 0o644)
			out2, _ := a.Push(providers)
			if !bytes.Equal(out1, out2) { t.Error("push is not idempotent") }
			if !strings.Contains(string(out1), "sk-test-KEEP") { t.Error("in-file secret was dropped") }
			if !strings.Contains(string(out1), `"custom"`) { t.Error("tool-specific field was lost") }
			for _, p := range providers {
				if strings.Contains(fmt.Sprint(p), "sk-test-KEEP") { t.Error("secret leaked into canonical form") }
			}
		})
	}
}
```
   kilocode は JSONC を受けるので seed は有効な JSON のままで通る。落ちたら実装のバグかテストの前提の誤りかを切り分け、直し方が自明でなければ止めて報告する。
2. `CONTRIBUTING.md`（日本語）: 開発の始め方（`make check`）、コミット規約（英語の Conventional Commits）、CHANGELOG の更新、秘密の扱いの原則（値を出さない・中継しない）、**アダプタ追加手順**を書く:
   1. `internal/adapter/<tool>.go` に `Adapter` を実装する（`Name` / `Path` / `Pull` / `Push` / `Project`。共通処理は `pullDocument` / `pushDocument` / `projectProviders` を呼ぶ）。
   2. `adapter.Get` の `switch` と `Names()` に追加する。
   3. 契約テストが自動で走ることを確認する。
   4. README の「対応ツール」に追加する。
3. 経緯や履歴は書かない（最新仕様のみ）。

### 注意
- `SECURITY.md` は 03 で作成済み。ここでは作らない。
- テスト内の秘密はダミー文字列（`sk-test-...`）のみ。
