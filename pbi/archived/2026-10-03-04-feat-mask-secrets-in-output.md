# PBI: diff・status・プレビュー出力で秘密の値をマスクする

## ユーザーストーリー
provsync の利用者として、`diff` や `status` の出力に API キーなどの秘密が表示されないでほしい、なぜなら画面共有・ターミナルログ・CI ログ・Issue への貼り付けから秘密が漏れるのを防ぎたいから

## 優先度
- 順位: 04 / 19
- RICEスコア: 12.8（Reach=8 / Impact=2 / Confidence=80% / Effort=1）
- 根拠: CHANGELOG 0.2.1 の README 追記により、`diff`・`status` が秘密を表示しうることが既知の制約として残っている。情報漏えいのリスクを下げる効果が大きいため、スコア順では init（05）の次だが、利用者判断により init より前倒しした。秘密を「中継しない」という設計原則を出力にも貫く修正

## 設計方針
- マスクは出力層（`internal/diff` の描画と cli の表示）でのみ行う。Plan と書き込み内容は一切変えない（ファイルの実データは保持される）
- 対象キーは pull が「秘密らしい」と判定する既存の判定（`apiKey`, `token` など）を共有し、判定を二重に持たない
- 値は `********` に置換するが、「変更があったか」は分かるようにする（変更前後で値が異なるときは `(changed)` を併記）

## BDD受け入れシナリオ
Scenario: diff で秘密が伏せ字になる
  Given kilocode の設定に `apiKey` が直書きされている
  When  `provsync diff kilocode opencode` を実行する
  Then  `apiKey` の値は `********` と表示され、実値は出力に含まれない

Scenario: 秘密が変わる場合は変更の事実だけ分かる
  Given push により in-file の秘密キーの値が変わる Plan がある
  When  プレビューを表示する
  Then  `apiKey: ******** (changed)` のように、値を出さずに変更を示す

Scenario: 書き込まれる内容はマスクされない
  Given 秘密を含むファイルに `--write` で push する
  When  書き込みが完了する
  Then  ファイルの秘密の値は元のまま保持される

Scenario: 明示すれば実値を表示できる
  Given 手元のターミナルで確認したい
  When  `--show-secrets` を付けて `diff` を実行する
  Then  実値が表示され、先頭に警告が出る

## 受け入れ基準
- [x] `diff`・`status`・`init`・`push`・`pull`・`sync` のプレビューすべてでマスクされる
- [x] 秘密判定ロジックを 1 か所に集約し、pull と出力層で共有する
- [x] バックアップ・中央設定・書き込み内容には影響しない
- [x] テストで「実値の文字列が出力に含まれない」ことを全コマンドで検査する
- [x] README の「diff・status での秘密の表示に関する注意」を最新仕様に書き換える

## テスト戦略
- E2E: 秘密入りの実ファイルで各コマンドの標準出力を grep し、実値が現れないこと
- 統合: `cli.Run --root <tmpdir>` で 4 シナリオ
- 単体: 秘密キー判定、ネストした秘密（`options.apiKey`）のマスク

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/adapter/adapter.go` の `secretLike` / `decodeProvider` / `carryOverSecrets`、`internal/cli/cli.go` の `cmdDiff` / `renderPreview` / `renderSemantic`、`internal/diff/diff.go` の `Unified`、`cli_test.go` の `TestPushPreservesToolConfigSecrets`。

### 事実の整理
- 秘密が画面に出る経路は `cmdDiff` が出す unified diff のみ。`Unified(c.Path, c.Before, c.After, 3)` はファイルの生の行をそのまま出す。ツール設定に直書きされた `"apiKey": "sk-..."` が文脈行・変更行として現れる。
- `renderSemantic`（意味差分）はキー名しか出さないので変更不要。
- 書き込む内容（`c.After`）は絶対に変えない。マスクは表示用文字列だけに適用する。

### 手順
1. 秘密判定を 1 か所に集約する。新規パッケージ `internal/secret/secret.go`:

```go
// Package secret は秘密情報らしいキーの判定と、表示用のマスクを提供する。
package secret

import (
	"regexp"
	"strings"
)

func IsKey(key string) bool {
	switch strings.ToLower(key) {
	case "apikey", "api_key", "token", "secret", "password", "accesstoken", "access_token":
		return true
	}
	return false
}

var jsonStringField = regexp.MustCompile(`("([^"\\]|\\.)*")(\s*:\s*)("([^"\\]|\\.)*")`)

// MaskLines は JSON 風の行 "key": "value" のうち key が秘密らしいものの value を伏せる。
func MaskLines(text string) string {
	return jsonStringField.ReplaceAllStringFunc(text, func(m string) string {
		sub := jsonStringField.FindStringSubmatch(m)
		key := strings.Trim(sub[1], `"`)
		if !IsKey(key) {
			return m
		}
		return sub[1] + sub[3] + `"********"`
	})
}
```
   `adapter.secretLike` の本体を `return secret.IsKey(key)` に置き換える（判定の二重管理をなくす）。
2. `internal/secret/secret_test.go`: `"apiKey": "sk-abc"` → `"apiKey": "********"`、`"name": "x"` は不変、`"options": {"token": "t1"}` の `t1` が消える、エスケープ入り値 `"apiKey": "a\"b"` も 1 値として伏せる。
3. `options` に `showSecrets bool` を足し、`registerFlags` に `--show-secrets` を追加。
4. `cmdDiff` の `fmt.Fprint(o.out, d)` を次に変える。

```go
if !o.showSecrets {
	d = secret.MaskLines(d)
} else {
	fmt.Fprintln(o.out, "警告: --show-secrets により秘密の値をそのまま表示しています")
}
fmt.Fprint(o.out, d)
```
5. 変更の事実を示す（任意の第 2 段階）: マスク前の diff を行ごとに走査し、`-` 行と `+` 行で同じキーの生の値が異なるなら、`+` 行の末尾に ` (changed)` を付けてからマスクする。難しければ第 1 段階だけで完了としてよい（その旨を CHANGELOG に書かない。仕様のみ書く）。
6. `cli_test.go` に追加:
   - `TestDiffMasksSecrets`: opencode の fixture ファイルの provider `llm-02` に `"apiKey": "sk-test-SECRET-123"` を足して書き直し、`diff kilocode opencode` を実行。出力に `sk-test-SECRET-123` が含まれず、`********` が含まれる。
   - `TestDiffShowSecrets`: `--show-secrets` 付きなら含まれ、警告も出る。
   - `TestPushWritePreservesSecretValue`: マスクを入れても `push opencode --write` 後のファイルに実値が残る。
7. README の「diff・status での秘密の表示に関する注意」を、最新仕様（既定でマスク、`--show-secrets` で解除）に書き換える。

### 注意
- `status` / `list` / プレビュー（`renderSemantic`）は値を出さない仕様なのでコード変更不要。ただしテストで「実値の文字列が全コマンドの出力に出ない」ことを確認する。
- 正規表現で扱えない形（配列内の秘密、複数行文字列）は対象外。本 PBI の範囲を広げない。
- マスクは表示層のみ。`plan.FileChange` の `Before` / `After` を加工しない。
