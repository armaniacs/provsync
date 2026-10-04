# PBI: JSONC パーサ・アダプタのファズテストとゴールデンテスト

## ユーザーストーリー
メンテナとして、壊れた・珍しい入力に対するパーサとアダプタの堅牢性をテストで保証したい、なぜなら設定ファイルを全文再シリアライズするツールで、入力の取りこぼしは利用者データの破損に直結するから

## 優先度
- 順位: 14 / 19
- RICEスコア: 2.4（Reach=3 / Impact=1 / Confidence=80% / Effort=1）
- 根拠: 品質上の価値は高いが、顕在化している不具合はない。機能系が出揃った後に回す

## BDD受け入れシナリオ
Scenario: 任意入力でも panic しない
  Given ランダムに生成した JSONC 文字列がある
  When  `StripJSONC` に与える
  Then  panic せず、有効な JSON ならその意味が保存される

Scenario: 文字列内のコメント記号を壊さない
  Given 文字列値に `//` や `/*` を含む設定がある
  When  JSONC を処理して pull と push を往復する
  Then  文字列値は 1 文字も変わらない

Scenario: 出力形式が意図せず変わらない
  Given 代表的な kilocode / opencode の入力ファイルがある
  When  push の出力を生成する
  Then  保存済みのゴールデンファイルとバイト単位で一致する

## 受け入れ基準
- [x] `FuzzStripJSONC`（標準の `testing.F`）を追加し、シードに既知の厄介例を入れる
- [x] adapter の往復（pull→push）が冪等であることをテーブルテストで確認
- [x] `testdata/` のゴールデン比較、更新用の `-update` フラグ
- [x] `make fuzz`（短時間）を追加

## テスト戦略
- E2E: 対象外
- 統合: アダプタの往復冪等性とゴールデン
- 単体: StripJSONC のファズ・境界値

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/jsonc/jsonc.go` と `jsonc_test.go`、`internal/adapter/adapter_test.go`。

### 事実の整理
`StripJSONC` は行コメント（`//`）と末尾カンマだけを扱う。ブロックコメント（`/* */`）は対象外。性質として守るべきことは「文字列リテラル内の `//` と `,` を壊さない」「有効な JSON を入力したら、同じ意味の JSON が出る」。

### 手順
1. `internal/jsonc/fuzz_test.go`:

```go
package jsonc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzStripJSONC(f *testing.F) {
	seeds := []string{
		`{"a": "http://x/y"}`,
		`{"a": [1, 2,],}`,
		"{\n  // c\n  \"a\": 1, // t\n}",
		`{"a": "\\"}`,
		`{"a": "x, }"}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		out := StripJSONC(in) // panic しないこと
		if !json.Valid(in) {
			return
		}
		var a, b any
		if err := json.Unmarshal(in, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out, &b); err != nil {
			t.Fatalf("valid JSON became invalid: %q -> %q", in, out)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("meaning changed: %q -> %q", in, out)
		}
	})
}
```
2. 短時間実行: `go test -fuzz=FuzzStripJSONC -fuzztime=20s ./internal/jsonc`。失敗入力は `internal/jsonc/testdata/fuzz/` に保存されるので、**コミットする**（回帰テストとして働く）。失敗したら `StripJSONC` の不具合の可能性が高い。直し方が自明でなければ止めて報告する。
3. `Makefile` に追加: `fuzz:` → `$(GO) test -fuzz=FuzzStripJSONC -fuzztime=20s ./internal/jsonc`（`.PHONY` にも）。
4. 往復冪等テスト `internal/adapter/roundtrip_test.go`: 各ツール名（`Names()`）について、一時ディレクトリに最小のツール設定を書き、`a.Pull()` → `a.Push(providers)` の出力を同じパスへ書いて、もう一度 `Push` すると出力が同一バイト列（冪等）になることを確認する。
5. ゴールデン: `internal/adapter/testdata/<tool>_push.golden` を作り、`var update = flag.Bool("update", false, "update golden files")` を使うテストで比較する。`go test ./internal/adapter -update` でのみ更新。初回は `-update` で生成して中身を目で確認してからコミットする。

### 注意
- ファズは `make check` に含めない（時間がかかるため）。通常の `go test` ではシード（とコミット済みの失敗入力）だけが実行される。
- ゴールデンは更新時に差分を必ず目で確認する。盲目的に `-update` しない。
