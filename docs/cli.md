# CLI リファレンス

## コマンド

| コマンド | 説明 |
|---|---|
| `list` | 対応ツールと設定パスを表示 |
| `status [tool...]` | ツールとセントラル設定の同期状態を表示 |
| `init [tool]` | 初回セットアップ(セントラル設定を作る)。既存のセントラル設定は上書きしない |
| `pull <tool>` | ツール設定をセントラル設定へ取り込む |
| `push <tool>` | セントラル設定をツール設定へ反映する |
| `sync --from <a> --to <b>` | a を取り込み b へ反映する(`--from` 省略時はセントラル設定をそのまま使う) |
| `diff <from> <to>` | from を to に適用した場合の差分(意味差分 + 統合 diff)を表示 |
| `undo [id]` | 直前または指定操作を復元する(`--list` で履歴、`--prune --keep <n>` で掃除) |
| `doctor` | 環境を診断する(存在・構文・`apiKeyEnv`・権限。通信しない) |
| `check` | 各 provider の API 到達可否を確認する(通信するのはこのコマンドだけ) |
| `completion <shell>` | bash / zsh / fish 用の補完スクリプトを出力 |
| `--version` / `version` | バージョンを表示 |

## フラグ

| フラグ | 既定 | 説明 |
|---|---|---|
| `--write` | `false` | 変更をファイルへ書き込む(既定はプレビュー) |
| `--provider <p>` | 全 provider | 対象 provider を限定(カンマ区切り・繰り返し可) |
| `--no-backup` | `false` | バックアップを記録しない(非推奨)。書き込みはマーカー操作として履歴に残る |
| `--show-secrets` | `false` | `diff` の出力で秘密の値をそのまま表示する(非推奨)。警告が表示される |
| `--root <dir>` | `$HOME` | パス解決の基準ディレクトリを差し替える(テスト用) |
| `--from` / `--to` | — | `sync` の取り込み元 / 反映先 |
| `--list` | `false` | `undo` の履歴を表示 |
| `--prune` / `--keep <n>` | `false` / `20` | `undo` の履歴を掃除する / 残す件数 |
| `--json` | `false` | `list` / `status` / `diff` を JSON で出力する |
| `--exit-code` | `false` | `status` で差分があるとき終了コード 3 で終了する |
| `--strict` | `false` | エイリアス未定義のモデル名があるときエラーにする(push) |
| `--version` | `false` | バージョンを表示 |
| `--help` / `-h` | `false` | ヘルプを表示 |

- フラグは位置引数の後にも置ける(`provsync push opencode --write` のように後置できる)。
- サブコマンド別ヘルプは `provsync <command> --help` で表示できる。

## 終了コード

| 終了コード | 意味 |
|---|---|
| `0` | 成功(差分なし) |
| `1` | 実行時エラー(設定の読み込み失敗など) |
| `2` | 使い方の誤り(未知のコマンド、引数の数違いなど) |
| `3` | 差分あり(`status --exit-code` のみ) |

エラーと警告は stderr、通常出力は stdout へ出る。

## 機械可読出力(--json)

`list` / `status` / `diff` に `--json` を付けると、標準出力は JSON のみになる(警告は stderr へ)。スキーマは `schemaVersion: 1`。

```json
{
  "schemaVersion": 1,
  "central": { "path": "~/.config/provsync/config.json", "exists": true, "providers": 2 },
  "tools": [
    {
      "name": "kilocode",
      "path": "~/.config/kilo/kilo.jsonc",
      "exists": true,
      "providers": 2,
      "warnings": ["秘密情報らしいフィールド ..."],
      "drift": ["llm-01: ツールに無い"],
      "driftEntries": [{ "provider": "llm-01", "op": "not-in-tool" }]
    }
  ]
}
```

- `status --json`: 上の形。`drift` が空 = 差分なし。`driftEntries` は drift の構造化版(additive)で、`op` は固定集合 `not-in-tool` / `not-in-central` / `drift` のいずれか。対話ツール(TUI など)は文言の逆解析ではなく `driftEntries` を使うことを推奨。
- `list --json`: `warnings` / `drift` を除いた形。
- `diff --json`: `{"schemaVersion": 1, "changes": [{"tool", "path", "semantic": [...], "diff": "マスク済み unified diff"}]}`。
- `status --exit-code`: 差分があるとき終了コード `3`、同期済みなら `0`。

## シェル補完

```bash
# zsh
provsync completion zsh > "${fpath[1]}/_provsync"

# bash
source <(provsync completion bash)

# fish
provsync completion fish | source
```
