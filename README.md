# llm-sync

`~/.config/kilo/kilo.jsonc` の provider エントリを `~/.config/opencode/opencode.json` へ移植する CLI ツール。

## 概要

- kilo 側の provider(`vs_inoue` / `sakura` / `vs` など)を opencode 側の `provider` に丸ごとコピーする。
- 既定はプレビューのみ。`--write` を付けたときだけファイルを更新する。
- 書き込み時は `opencode.json.bak` を作成し、一時ファイル経由のアトミック置換で更新する。
- source は JSONC(行コメント・末尾カンマ)に対応。target の対象外 provider はそのまま保持する。

## 必要環境

- Go 1.22 以上(標準ライブラリのみ使用、外部依存なし)

## ビルド

```bash
go build -o llm-sync .
```

## 使い方

```bash
# プレビュー(ファイルは変更しない)
./llm-sync

# 実際に書き込む(バックアップ付きアトミック更新)
./llm-sync --write
```

### フラグ

| フラグ | 既定値 | 説明 |
|---|---|---|
| `--source` | `$HOME/.config/kilo/kilo.jsonc` | 移植元の kilo 設定(JSONC) |
| `--target` | `$HOME/.config/opencode/opencode.json` | 移植先の opencode 設定(JSON) |
| `--write` | `false` | `true` のときだけ target を更新 |
| `--backup` | `true` | `--write` 時に `.bak` を作成 |
| `--providers` | `vs_inoue,sakura,vs` | 移植する provider キー(カンマ区切り) |

### 実行例

```bash
# 別のファイルを指定してプレビュー
./llm-sync --source ~/tmp/kilo.jsonc --target ~/tmp/opencode.json

# 特定の provider だけ移植
./llm-sync --providers sakura --write
```

出力例:

```
vs_inoue: 置換 (models 3件)
sakura: 置換 (models 10件)
vs: 置換 (models 4件)
(preview only; use --write to apply)
```

## 挙動の詳細

- 対象 provider はキー名をそのまま使い、既存エントリを丸ごと上書きする。
- target の `provider` にしか存在しないキーは保持される。
- source に指定した provider が無い場合、ファイルが読めない場合、JSON が不正な場合はエラー終了(終了コード非 0)。
- 出力 JSON は 2 スペースインデントで全体が再整形される(トップレベルキーはアルファベット順)。

## テスト

```bash
go test ./...
gofmt -l .
go vet ./...
```

## 構成

```
main.go                       CLI、ファイル I/O、バックアップ、アトミック書き込み
main_test.go                  run() の統合テスト
internal/syncer/syncer.go     provider マージ(純粋関数)
internal/syncer/jsonc.go      JSONC → 純 JSON の前処理
internal/syncer/*_test.go     単体テスト
```

## 設計・計画ドキュメント

- 設計: `docs/superpowers/specs/2026-10-01-kilo-to-opencode-provider-port-design.md`
- 実装計画: `docs/superpowers/plans/2026-10-01-kilo-to-opencode-provider-port.md`