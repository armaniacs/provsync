# kilo → opencode Provider Port Tool 設計

日付: 2026-10-01
対象ディレクトリ: `/Users/y-araki/Playground/SakuraTools/llm-sync`

## 目的

`~/.config/kilo/kilo.jsonc` の `provider` から `vs_inoue` / `sakura` / `vs` の 3 エントリを取り出し、
`~/.config/opencode/opencode.json` の `provider` へ移植する Go 製 CLI ツールを作る。

## スコープ

- 単一の Go コマンド `llm-sync` を提供する。
- 既定動作は変更差分のプレビュー。`--write` 指定時のみ対象ファイルを更新する。
- 移植対象は provider エントリの内容そのもの(`npm` / `name` / `options` / `models` および
  `reasoning` / `modalities` などの任意フィールドを含む)。
- opencode 側で対象キー以外の provider(`vsakura` / `vpn-sakura` など)はそのまま保持する。

## 非目標(YAGNI)

- 順序・整形の完全保持(全体再整形を許容する)。
- 双方向同期、ファイル監視、複数 config ペアの汎用対応。
- 詳細な JSON 行 diff。
- opencode 以外の出力形式。

## CLI 仕様

```
llm-sync [flags]

  --source     string   既定: $HOME/.config/kilo/kilo.jsonc
  --target     string   既定: $HOME/.config/opencode/opencode.json
  --write               既定: false。true のときだけファイルを更新
  --backup              既定: true。--write 時に .bak を作成
  --providers  string   既定: vs_inoue,sakura,vs(カンマ区切り)
```

- 既定実行はプレビューのみ。ファイルは一切変更しない。
- `--write` 時は `opencode.json.bak` を作成し、一時ファイルへ書いて `os.Rename` でアトミック置換する。
- 終了コード: 正常 0、エラー非 0。

## 処理フロー

1. `--source` を読み込む。source は JSONC(コメント・末尾カンマを含む)なので、
   JSONC → 純 JSON の前処理を行ってから `encoding/json` でパースする。
2. `--target` を読み込む。存在しなければエラー。
3. 対象 provider キーごとに source の `provider[key]` を取得する。無ければエラー終了。
4. target の `provider[key]` をエントリ丸ごと上書きする(キー名は同一)。
   target にしか無いキーはそのまま保持する。
5. 結果を `json.MarshalIndent(v, "", "  ")` で直列化する(トップレベルキーは
   アルファベット順にソートされる。これは許容済み)。
6. プレビュー出力、または `--write` ならバックアップ + アトミック書き込み。

## データモデル

provider の内容は任意フィールドを欠落させないため、`map[string]any` として扱う。

```go
type Config = map[string]any
// provider 部分は map[string]map[string]any として扱う
```

型付き struct は将来増えたフィールドを取りこぼすため採用しない。
`json.RawMessage` による外科的置換は、全体再整形を許容するため過剰として採用しない。

## コンポーネント構成

```
llm-sync/
  go.mod
  main.go                       # CLI / フラグ解析 / 入出力
  internal/syncer/syncer.go     # 変換ロジック(純粋関数)
  internal/syncer/jsonc.go      # JSONC → JSON 前処理
  internal/syncer/syncer_test.go
```

### インターフェース

```go
// Merge は source の provider から keys を抜き出し、target の provider を上書きして返す。
// target の他キーは保持する。key が source に無ければエラー。
func Merge(source, target map[string]any, keys []string) (map[string]any, error)

// StripJSONC は行コメントと末尾カンマを除去して純 JSON 化する。
func StripJSONC(b []byte) []byte
```

`Merge` を純粋関数にして単体テストしやすくする。

## 差分プレビュー

- 各対象 provider について `sakura: 置換 (models 8件)` のように要約表示する。
- `--write` 時も同じサマリを表示する。
- 変更が無い場合も正常終了(0)。

## エラー処理

| 状況 | 挙動 |
|---|---|
| source が読めない/JSON 不正 | エラーメッセージ + 非 0 終了 |
| target が読めない/JSON 不正 | エラーメッセージ + 非 0 終了 |
| 対象 provider が source に無い | エラーメッセージ + 非 0 終了 |
| バックアップ/書き込み失敗 | エラーメッセージ + 非 0 終了(元ファイルは無傷) |

## テスト

- JSONC 前処理: 行コメント `//`、末尾カンマの除去。
- `Merge`: 対象 3 キーが上書きされる。
- `Merge`: target の他プロバイダ(`vsakura` 等)が保持される。
- `Merge`: source に対象キーが無ければエラー。
- ゴールデン: サンプル source/target を一時ディレクトリに置き、変換後 JSON を検証。

## 想定される注意点

- `sakura` は既存 opencode.json に存在するため内容が置換される。`vs` は新規キーとして追加される。
- `vsakura` は残るが `vs` と baseURL が重複する。これは上書き方針(同一キー名)の帰結であり、
  既存 `vsakura` を削除・改名しない。