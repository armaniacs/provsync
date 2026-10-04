# PBI: 初回セットアップ用 init コマンドを追加する

## ユーザーストーリー
初めて provsync を使う人として、`provsync init` で迷わず中央設定を作りたい、なぜなら「どのツールから取り込み、次に何を打つか」が分からないと最初の一歩で挫折するから

## 優先度
- 順位: 05 / 19
- RICEスコア: 16（Reach=10 / Impact=2 / Confidence=80% / Effort=1）
- 根拠: 導入体験に直結し価値が最大。04（出力の秘密マスク）を先に入れてから公開する方針のため 05 とした。工数は 01・02 より大きいため後続。`pull` の Plan を再利用するので新規ロジックは小さい

## 設計方針
- `init [tool]` は初回専用。内部では `pull` と同じ Plan を構築して再利用し、「何が変わるか」を別計算しない
- 既定はプレビュー、`--write` で作成（既存の安全方針を踏襲）
- `provsync init kilocode --write` は `provsync pull kilocode --write` と等価な結果になる
- tool 省略時は、設定ファイルが存在する対応ツールを検出して一覧し、候補が 1 つならそれを使い、複数なら選び方を案内して終了する（対話入力は不要。非対話でも動く）
- 終了時に「次にやること」（env 変数の設定、`status`、`push`）を表示する

## BDD受け入れシナリオ
Scenario: kilocode から初回の中央設定を作る
  Given 中央設定が存在せず、kilocode の設定ファイルがある
  When  `provsync init kilocode --write` を実行する
  Then  中央設定が作成され、取り込んだ provider 数と次の手順が表示される

Scenario: プレビューでは何も書かない
  Given 中央設定が存在しない
  When  `provsync init kilocode` を実行する（--write なし）
  Then  作成予定のパスと取り込み内容が表示され、ファイルは作られず、`--write` を付けて再実行する案内が出る

Scenario: ツール省略時に候補を検出する
  Given kilocode と opencode の両方の設定ファイルがある
  When  `provsync init` を実行する
  Then  検出したツールの一覧が表示され、`provsync init <tool>` の指定を促す

Scenario: すでに初期化済みなら上書きしない
  Given 中央設定がすでに存在する
  When  `provsync init kilocode --write` を実行する
  Then  エラーで終了し、既存設定は変更されず、日常の更新には `pull` を使うよう案内される

Scenario: 設定ファイルが 1 つも見つからない
  Given 対応ツールの設定ファイルが存在しない
  When  `provsync init` を実行する
  Then  探したパスの一覧と、ツールを先に設定する旨が表示される

Scenario: 秘密情報の警告
  Given kilocode の設定に apiKey が直書きされている
  When  `provsync init kilocode --write` を実行する
  Then  秘密は中央設定に書かれず、`apiKeyEnv` へ移す手順が警告として表示される

## 受け入れ基準
- [x] `init [tool]` を `reorder` 対応のサブコマンドとして追加し、`usage` に載せる
- [x] 実行内容は `internal/plan` の Plan を再利用し、`--write` 時もバックアップ・atomic 書き込み規則に従う
- [x] 既存の中央設定は上書きしない（`--force` は提供しない）
- [x] 出力は日本語。次の手順（`status` → `push` → 失敗時 `undo`）を含む
- [x] 秘密は中継しない（pull と同じ除去・警告）

## テスト戦略
- E2E: 空の HOME から init → status → push の通し
- 統合: `cli.Run --root <tmpdir>` で上記 6 シナリオ
- 単体: ツール検出ロジック、既存設定の判定

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README に「はじめに」節を追加、CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/cli/cli.go` の `cmdPull`（そのままの手本）、`buildCentralChange`、`applyOrPreview`、`loadCentralOrNew`。`cli_test.go` の `TestPullWriteCreatesCentral`。02 と 04 が済んでいること。

### 設計の確認
`init` は「初回専用の pull」。変更の計算は `buildCentralChange` を再利用し、`applyOrPreview` に渡す。新しい差分計算を書かない。`--force` は提供しない。

### 手順
1. `Run` の `switch` に `case "init": return cmdInit(opts, rest)` を足す。`usage` のコマンド一覧に `init [tool]  初回セットアップ(中央設定を作る)` を足す。02 で書いた「作成案内」の行を `provsync init <tool> --write で作成します` に書き換える。
2. `cmdInit` の処理順（上から順にそのまま実装）:

```go
func cmdInit(o *options, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("使い方: provsync init [tool]")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	central := root.CentralConfigPath()
	if _, err := os.Stat(central); err == nil {
		return fmt.Errorf("すでに初期化されています: %s\n日常の更新には provsync pull <tool> を使ってください", central)
	}

	tool := ""
	if len(args) == 1 {
		tool = args[0]
	} else {
		found := detectTools(root) // 設定ファイルが存在するツール名のスライス
		switch len(found) {
		case 0:
			// 探したパスを全部出して error を返す
		case 1:
			tool = found[0]
			fmt.Fprintf(o.out, "検出したツール: %s\n", tool)
		default:
			// 検出した一覧を出し「provsync init <tool>」で指定するよう案内して return nil
		}
	}
	// 以降は cmdPull と同じ: Get → Pull → 警告表示 → FilterProviders → buildCentralChange → applyOrPreview
}
```
   - `detectTools(root adapter.Root) []string`: `adapter.Names()` を回し、`a.Path()` が `os.Stat` で存在するものを返す。
   - 0 件のとき: `対応ツールの設定ファイルが見つかりません。探した場所:` に続けて各ツールの `a.Path()` を 1 行ずつ出し、`ツールを先に設定してから再実行してください` を添えて `errors.New` を返す。
3. `applyOrPreview` の呼び出し後の案内:
   - `--write` なし（`!o.write`）: `次に: provsync init <tool> --write で中央設定を作成します` を出す。
   - `--write` あり、かつ書き込みが行われた: 次を出す。

```
次の手順:
  1. provsync status              同期状態を確認する
  2. provsync push <他のツール>    他のツールへ反映する(まずプレビュー)
  3. 問題があれば provsync undo で元に戻せます
```
   - `pulled` の取り込み件数が 0 のとき: `取り込める provider がありません` と出す（中央設定は作らない: `p.Changed()` が false のときは書かれない既存挙動に従う）。
   - 取り込み時の警告が 1 件以上あったとき: `秘密は中央設定に保存されません。環境変数名を中央設定の apiKeyEnv に設定してください` を追加で出す。
4. `cli_test.go` に追加（`setup(t)` を使う。中央設定は最初は無い）:
   - `TestInitWriteCreatesCentral`: `init kilocode --write` で中央設定ができ、出力に `次の手順` が含まれる。
   - `TestInitPreviewDoesNotWrite`: `init kilocode` で中央設定ができず、`--write` の案内が出る。
   - `TestInitDetectsSingleTool`: kilocode だけ残す（`os.Remove(f.opencode)`）→ `init --write` で kilocode が自動選択される。
   - `TestInitMultipleToolsListsCandidates`: 2 つあるとき `init` で候補が出て、中央設定は作られない。
   - `TestInitAlreadyInitializedErrors`: `pull kilocode --write` 後の `init kilocode --write` が error（メッセージに `pull` を含む）、中央設定は変わらない。
   - `TestInitNoToolsErrors`: `t.TempDir()` を root にして `init` が error、出力または error 文に探したパスを含む。
   - `TestInitSecretWarning`: kilocode の fixture に `"apiKey": "sk-test-1"` を足して `init kilocode --write` → 中央設定のファイルに `sk-test-1` が無い、警告が出る。
5. README に「はじめに」節を追加（日本語・English 両方）: `provsync init kilocode` でプレビュー → `--write` で作成 → `status` → `push`。

### 注意
- `os.Stat(central)` が成功したら必ずエラー。上書きの分岐を作らない。
- ツール名の別名（`kilo`）は `adapter.Get` が解決するので `init kilo` も動く。自前で変換しない。
