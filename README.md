# provsync

**複数の LLM ツールの provider 設定を、たった一つの中央設定に同期する**
*Sync provider entries across your LLM tools into one central canonical config.*

[日本語](#日本語) | [English](#english)

## 日本語

### 概要

provsync は、複数の LLM コーディングツールがそれぞれ独自の形式で持つ `provider` 設定を、中央のカノニカル設定を唯一の正(single source of truth)として同期する CLI ツール。`pull` でツールから中央へ取り込み、`push` で中央からツールへ反映する。Go 標準ライブラリのみで実装され、外部依存はない。

### 安全設計

- 既定はプレビューのみ。`--write` を付けたときだけファイルが変わる。
- 書き込みの直前に、影響する全ファイルをタイムスタンプ付きの 1 操作として自動バックアップ。
- `undo` は復元前の現状も新しい操作として記録するため、undo 自体を undo できる(redo 可能)。
- プレビュー・`diff`・適用・バックアップはすべて同一の変更計画(Plan)から生成される。「diff で見た内容」=「書かれる内容」=「undo で戻る内容」。
- バイト単位で同一の書き込みはスキップされる(冪等)。変更が無ければ `変更はありません` と表示されるだけ。

### 対応ツール

macOS / Linux 対応。Windows は非対応。

| ツール | 設定ファイル | 形式 |
|---|---|---|
| kilocode(別名 `kilo`) | `~/.config/kilo/kilo.jsonc` | JSONC(行コメント・末尾カンマ対応) |
| opencode | `~/.config/opencode/opencode.json` | JSON |

- 中央設定: `~/.config/provsync/config.json`
- 状態(バックアップと履歴): `~/.local/state/provsync/`
- パスは `XDG_CONFIG_HOME` / `XDG_STATE_HOME` を尊重。

### インストール

Go 1.25.14 以上。macOS / Linux 対応(Windows は非対応)。

```bash
go install github.com/armaniacs/provsync@latest
```

Go を使わない場合は、Releases からビルド済みバイナリを入手する。

```bash
# macOS (Apple Silicon) の例
curl -sLO https://github.com/armaniacs/provsync/releases/latest/download/provsync_<ver>_darwin_arm64.tar.gz
tar xzf provsync_*_darwin_arm64.tar.gz
sudo mv provsync /usr/local/bin/
```

ソースからビルドする場合:

```bash
git clone https://github.com/armaniacs/provsync.git
cd provsync
make build    # bin/provsync
```

### はじめに

初めて使うときは `provsync init` で中央設定を作る。

```console
# 取り込み元のツールを指定(まずはプレビュー)
$ provsync init kilocode

# --write で中央設定を作成
$ provsync init kilocode --write

# 同期状態を確認して、他のツールへ反映する
$ provsync status
$ provsync push opencode --write
```

ツールを省略すると、設定ファイルが存在するツールを検出する。候補が複数ある場合は `provsync init <tool>` で指定する。中央設定が既にある場合は上書きしないため、日常の更新には `pull` を使う。

### クイックスタート

```console
# kilocode の provider を中央設定へ取り込む(まずはプレビュー)
$ provsync pull kilocode
central: /home/you/.config/provsync/config.json
  llm-01: 追加
  llm-02: 追加
(プレビューのみ; 適用するには --write)

# --write で適用。書き込みの直前にバックアップが記録される
$ provsync pull kilocode --write
central: /home/you/.config/provsync/config.json
  llm-01: 追加
  llm-02: 追加
バックアップ: 20261002T093012-3fa1
書き込み: /home/you/.config/provsync/config.json

# 中央設定を opencode へ反映する
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  llm-01: 追加
  llm-02: 追加
バックアップ: 20261002T093045-8c2d
書き込み: /home/you/.config/opencode/opencode.json

# 同じ状態への再書き込みは冪等
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  変更なし
変更はありません
```

同期状態の確認:

```console
$ provsync status
中央設定: /home/you/.config/provsync/config.json (2 providers)
kilocode  /home/you/.config/kilo/kilo.jsonc (2 providers)
  差分なし
opencode  /home/you/.config/opencode/opencode.json (3 providers)
  警告: 秘密情報らしいフィールド "options.apiKey" を検出しました。秘密は仲介しないため取り込みません
  groq: 中央に無い
```

ツール側にだけ存在する provider(上の `groq`)は「中央に無い」と表示される。`push` が対象ツール設定内の管理対象外エントリを削除することはない。

差分の確認と取り消し:

```console
$ provsync diff kilocode opencode
central: /home/you/.config/provsync/config.json
  変更なし
  変更なし
opencode: /home/you/.config/opencode/opencode.json
  llm-01: 追加
  llm-02: 追加
--- a/home/you/.config/opencode/opencode.json
+++ b/home/you/.config/opencode/opencode.json
@@ -1,15 +1,44 @@
   {
 -  "theme": "dark",
   "provider": {
     "groq": {
 +      "models": {
 +        "llama-3.3-70b-versatile": {
 +          "name": "Llama 3.3 70B Versatile"
 +        }
 +      },
       "name": "Groq",
       "npm": "@ai-sdk/groq",
       "options": {
         "apiKey": "********"
 ...
 +  "theme": "dark"
 }

$ provsync undo
復元しました: 20261002T093045-8c2d (push opencode)
やり直し: provsync undo 20261002T100001-51b7
  復元: /home/you/.config/opencode/opencode.json
```

`undo` は `--write` を要求せず直接適用する。

### コマンドリファレンス

| コマンド | 説明 |
|---|---|
| `list` | 対応ツールと設定パスを表示 |
| `status [tool...]` | ツールと中央設定の同期状態を表示 |
| `init [tool]` | 初回セットアップ(中央設定を作る)。既存の中央設定は上書きしない |
| `pull <tool>` | ツール設定を中央設定へ取り込む |
| `push <tool>` | 中央設定をツール設定へ反映する |
| `sync --from <a> --to <b>` | a を取り込み b へ反映する(`--from` 省略時は中央設定をそのまま使う) |
| `diff <from> <to>` | from を to に適用した場合の差分(意味差分 + 統合 diff)を表示 |
| `undo [id]` | 直前または指定操作を復元する(`--list` で履歴、`--prune --keep <n>` で掃除) |
| `doctor` | 環境を診断する(存在・構文・`apiKeyEnv`・権限。通信しない) |
| `completion <shell>` | bash / zsh / fish 用の補完スクリプトを出力 |
| `--version` / `version` | バージョンを表示 |

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
| `--version` | `false` | バージョンを表示 |
| `--help` / `-h` | `false` | ヘルプを表示 |

- フラグは位置引数の後にも置ける(`provsync push opencode --write` のように後置できる)。
- サブコマンド別ヘルプは `provsync <command> --help` で表示できる。
- 終了コード: `0` 成功 / `1` 実行時エラー / `2` 使い方の誤り / `3` 差分あり(`status --exit-code` のみ)。

| 終了コード | 意味 |
|---|---|
| `0` | 成功(差分なし) |
| `1` | 実行時エラー(設定の読み込み失敗など) |
| `2` | 使い方の誤り(未知のコマンド、引数の数違いなど) |
| `3` | 差分あり(`status --exit-code` のみ) |

### 機械可読出力(--json)

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
      "drift": ["llm-01: ツールに無い"]
    }
  ]
}
```

- `status --json`: 上の形。`drift` が空 = 差分なし。
- `list --json`: `warnings` / `drift` を除いた形。
- `diff --json`: `{"schemaVersion": 1, "changes": [{"tool", "path", "semantic": [...], "diff": "マスク済み unified diff"}]}`。
- `status --exit-code`: 差分があるとき終了コード `3`、同期済みなら `0`。

### 中央設定

既知フィールド(`name` / `npm` / `baseURL` / `apiKeyEnv` / `models`)は正規化して保持し、ツール固有の未知フィールドは `x.<tool>` に名前空間化して保存する。同じツールへ `push` するときに復元されるため、pull → push の往復で失われない。

```json
{
  "providers": {
    "llm-01": {
      "apiKeyEnv": "LLM01_API_KEY",
      "baseURL": "https://llm-01.example.com/v1",
      "models": {
        "model-a": {
          "name": "Model A"
        },
        "model-b": {
          "name": "Model B"
        }
      },
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "x": {
        "kilocode": {
          "reasoning": true
        }
      }
    }
  },
  "version": 1
}
```

`version` は中央設定の形式バージョン(将来のマイグレーション用)。現行は `1`。

### モデルエイリアス

中央設定の `aliases` に、共通モデル名 → ツール名 → モデル ID の対応を書くと、`push` のときに各ツールが要求する ID 形式へ自動変換する。

```json
{
  "aliases": {
    "sonnet": {
      "kilocode": "claude-sonnet-4-5",
      "opencode": "anthropic/claude-sonnet-4-5"
    }
  },
  "providers": {
    "llm-01": {
      "models": { "sonnet": { "name": "Sonnet" } }
    }
  }
}
```

- 変換は push の描画前にだけ行う。`pull` は ID を書き換えない(往復で情報を失わない)。
- `aliases` にないモデル名は変換せず素通しする。
- 当該ツール向けの ID が未定義のエイリアスは警告して素通しする。`--strict` を付けるとエラーで終了し、何も書かない。
- 組み込みの対応表はない。対応表の正はユーザー定義の `aliases` で、pull しても消えない。

### バックアップと undo

- 書き込み直前に、影響する全ファイルを 1 操作としてバックアップする。
- 保存先は `~/.local/state/provsync/`:`index.json`(操作履歴)と `backups/<op-id>/`(SHA-256 検証付きのファイルスナップショット)。
- 最新 20 操作を保持し、古い操作は自動削除される。`PROVSYNC_KEEP` 環境変数で保持数を変更できる(1 以上の整数)。

```console
$ provsync undo --list
20261002T093045-8c2d  2026-10-02 09:30:45  push opencode
    /home/you/.config/opencode/opencode.json
20261002T093012-3fa1  2026-10-02 09:30:12  pull kilocode
    /home/you/.config/provsync/config.json
```

- `provsync undo` は直前の書き込みを、`provsync undo <id>` は指定操作を復元する。`--write` を要求せず直接適用する。
- undo は復元前の現状を新しい操作として記録し、やり直し用の ID を表示する(`やり直し: provsync undo <id>`)。
- `provsync undo --prune --keep <n>` で新しい n 件を残して履歴を掃除できる(削除件数が表示される)。
- `--no-backup` での書き込みはマーカー操作として履歴に記録される。後続の `undo` では「直近の書き込みはバックアップなしで行われたため、この undo はそれより前の状態に戻します」と警告される。
- 新規作成される中央設定と状態ディレクトリは 0600 / 0700 で作られる。既存ファイルの権限は変更されない。`status` は権限が緩い場合に `chmod` を案内する。
- dotfiles 管理のシンボリックリンクは維持される。書き込みはリンク先の実体に対して行われ、リンク自体が通常ファイルに置き換わることはない。リンク切れは書き込み前にエラーになる。`list` はリンク先を ` (symlink → 実体)` で表示する。
- `--write` と `undo` の復元区間は状態ディレクトリの排他ロック(flock)で保護される。並行実行は後発が待機し、タイムアウト(10 秒)すると「別の provsync が実行中」で終了する。プレビュー・`diff`・`status` はロックを取らない。プロセスが異常終了した場合、OS がロックを解放するため自動回復する。

### 秘密情報の扱い

- 中央設定が持つのは `apiKeyEnv`(環境変数名)のみ。実キーは仲介しない。
- `pull` 時、秘密情報らしいフィールド(`apiKey` / `api_key` / `token` / `secret` / `password` / `accessToken` / `access_token`。`options` 内も含む)は警告のうえ取り込まない:

```console
警告: 秘密情報らしいフィールド "options.apiKey" を検出しました。秘密は仲介しないため取り込みません
```

- `push` 時、対象ツール設定に既に存在する秘密フィールドはそのまま保持される(削除も中央への持ち出しもしない)。
- opencode には `apiKeyEnv` を書き出さない(秘密は `auth.json` で管理されるため)。
- `diff` の出力では、秘密情報らしいキー(`apiKey` / `token` など)の値を既定で `********` に伏せる。書き込まれるファイルの内容はマスクされない(表示のみ)。
- 手元で実値を確認したい場合は `--show-secrets` を付ける。実値が表示され、先頭に警告が出る。
- `status` / `list` はキー名の警告のみを表示し、値を出力しない。
- 脆弱性の報告方法は [SECURITY.md](SECURITY.md) を参照。

### 再直列化に関する注意

ツール設定は全体が再直列化される(2 スペースインデント・キー辞書順)。元のコメントや書式は失われ、内容が同じでも `diff` に書式差分が混ざることがある。

### 開発

```bash
make check    # fmt-check → vet → test
make build    # bin/provsync
make lint     # staticcheck
make vuln     # govulncheck
make test-race  # データ競合検出付きテスト
```

パッケージ構成: `main.go`(ディスパッチ)、`internal/cli`(サブコマンド)、`internal/model`(カノニカル表現)、`internal/adapter`(kilocode / opencode 変換)、`internal/store`(中央設定)、`internal/syncer`(マージ)、`internal/jsonc`(JSONC 前処理)、`internal/plan`(変更計画と意味差分)、`internal/diff`(統合 diff)、`internal/backup`(バックアップと復元)、`internal/fsutil`(atomic write・JSON 整形)。

- [CHANGELOG.md](CHANGELOG.md)
- [設計ドキュメント](docs/superpowers/specs/2026-10-02-provsync-multi-tool-sync-design.md)

## English

### Overview

provsync is a CLI that syncs the `provider` entries your LLM coding tools each keep in their own format, using a central canonical config as the single source of truth. `pull` imports from a tool into the central config; `push` reflects the central config back into a tool. Built on the Go standard library only, with no external dependencies.

### Safety Design

- Preview-only by default. Files change only with `--write`.
- Every write is preceded by an automatic timestamped backup of all affected files as a single operation.
- `undo` records the pre-restore state as a new operation, so undo itself can be undone (redoable).
- Preview, `diff`, apply, and backup are all generated from the same Plan. What you see in the diff is exactly what gets written — and what undo restores.
- Byte-identical writes are skipped (idempotent). If nothing changes, you just get `変更はありません` ("no changes").

### Supported Tools

macOS / Linux only. Windows is unsupported.

| Tool | Config file | Format |
|---|---|---|
| kilocode (alias `kilo`) | `~/.config/kilo/kilo.jsonc` | JSONC (line comments, trailing commas) |
| opencode | `~/.config/opencode/opencode.json` | JSON |

- Central config: `~/.config/provsync/config.json`
- State (backups and history): `~/.local/state/provsync/`
- `XDG_CONFIG_HOME` / `XDG_STATE_HOME` are respected.

### Installation

Go 1.25.14 or later. macOS / Linux only (Windows is unsupported).

```bash
go install github.com/armaniacs/provsync@latest
```

Without the Go toolchain, download a prebuilt binary from Releases.

```bash
# Example for macOS (Apple Silicon)
curl -sLO https://github.com/armaniacs/provsync/releases/latest/download/provsync_<ver>_darwin_arm64.tar.gz
tar xzf provsync_*_darwin_arm64.tar.gz
sudo mv provsync /usr/local/bin/
```

Or build from source:

```bash
git clone https://github.com/armaniacs/provsync.git
cd provsync
make build    # bin/provsync
```

### Getting Started

Run `provsync init` to create the central config the first time.

```console
# Specify the source tool (preview first)
$ provsync init kilocode

# Create the central config with --write
$ provsync init kilocode --write

# Check the sync status, then reflect into other tools
$ provsync status
$ provsync push opencode --write
```

With the tool omitted, provsync detects tools whose config files exist. With multiple candidates, specify the tool with `provsync init <tool>`. An existing central config is never overwritten; use `pull` for everyday updates.

### Quick Start

Note: CLI messages are in Japanese.

```console
# Import kilocode providers into the central config (preview first)
$ provsync pull kilocode
central: /home/you/.config/provsync/config.json
  llm-01: 追加
  llm-02: 追加
(プレビューのみ; 適用するには --write)

# Apply with --write; a backup is recorded just before writing
$ provsync pull kilocode --write
central: /home/you/.config/provsync/config.json
  llm-01: 追加
  llm-02: 追加
バックアップ: 20261002T093012-3fa1
書き込み: /home/you/.config/provsync/config.json

# Reflect the central config into opencode
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  llm-01: 追加
  llm-02: 追加
バックアップ: 20261002T093045-8c2d
書き込み: /home/you/.config/opencode/opencode.json

# Re-writing the same state is idempotent
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  変更なし
変更はありません
```

Check sync status:

```console
$ provsync status
中央設定: /home/you/.config/provsync/config.json (2 providers)
kilocode  /home/you/.config/kilo/kilo.jsonc (2 providers)
  差分なし
opencode  /home/you/.config/opencode/opencode.json (3 providers)
  警告: 秘密情報らしいフィールド "options.apiKey" を検出しました。秘密は仲介しないため取り込みません
  groq: 中央に無い
```

Providers that exist only in a tool (`groq` above) show as "not in central". `push` never deletes unmanaged entries in the target tool config.

Inspect changes and undo:

```console
$ provsync diff kilocode opencode
central: /home/you/.config/provsync/config.json
  変更なし
  変更なし
opencode: /home/you/.config/opencode/opencode.json
  llm-01: 追加
  llm-02: 追加
--- a/home/you/.config/opencode/opencode.json
+++ b/home/you/.config/opencode/opencode.json
@@ -1,15 +1,44 @@
  ...

$ provsync undo
復元しました: 20261002T093045-8c2d (push opencode)
やり直し: provsync undo 20261002T100001-51b7
  復元: /home/you/.config/opencode/opencode.json
```

`undo` applies directly; it does not require `--write`.

### Command Reference

| Command | Description |
|---|---|
| `list` | List supported tools and their config paths |
| `status [tool...]` | Show sync status between tools and the central config |
| `init [tool]` | First-time setup (creates the central config). Never overwrites an existing one |
| `pull <tool>` | Import a tool config into the central config |
| `push <tool>` | Reflect the central config into a tool config |
| `sync --from <a> --to <b>` | Pull from a and push to b (`--from` optional: uses the central config as-is) |
| `diff <from> <to>` | Show what applying from to to would change (semantic + unified diff) |
| `undo [id]` | Restore the last (or given) operation (`--list` for history, `--prune --keep <n>` to clean up) |
| `doctor` | Diagnose the environment (existence, syntax, `apiKeyEnv`, permissions; no network) |
| `completion <shell>` | Print a completion script for bash / zsh / fish |

| Flag | Default | Description |
|---|---|---|
| `--write` | `false` | Write changes to files (preview by default) |
| `--provider <p>` | all providers | Restrict to specific providers (comma-separated, repeatable) |
| `--no-backup` | `false` | Skip backup recording (discouraged). The write is still recorded as a marker operation |
| `--show-secrets` | `false` | Show raw secret values in `diff` output (discouraged). A warning is printed |
| `--root <dir>` | `$HOME` | Override the base directory for path resolution (testing) |
| `--from` / `--to` | — | Source / target tools for `sync` |
| `--list` | `false` | Show `undo` history |
| `--prune` / `--keep <n>` | `false` / `20` | Clean up `undo` history / ops to keep |
| `--json` | `false` | Output `list` / `status` / `diff` as JSON |
| `--exit-code` | `false` | Exit with code 3 when `status` detects drift |
| `--version` | `false` | Show the version |
| `--help` / `-h` | `false` | Show help |

- Flags may appear after positional arguments (e.g. `provsync push opencode --write`).
- Per-command help is available via `provsync <command> --help`.
- Exit codes: `0` success / `1` runtime error / `2` usage error / `3` drift (`status --exit-code` only). Errors and warnings go to stderr; normal output to stdout.

| Exit code | Meaning |
|---|---|
| `0` | Success (no drift) |
| `1` | Runtime error (config load failure, etc.) |
| `2` | Usage error (unknown command, wrong argument count, etc.) |
| `3` | Drift detected (`status --exit-code` only) |

### Machine-readable output (--json)

`list` / `status` / `diff` accept `--json`; stdout then carries JSON only (warnings go to stderr). The schema carries `schemaVersion: 1`.

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
      "warnings": ["secret-like field ..."],
      "drift": ["llm-01: ツールに無い"]
    }
  ]
}
```

- `status --json`: the shape above; empty `drift` = no drift.
- `list --json`: the same shape without `warnings` / `drift`.
- `diff --json`: `{"schemaVersion": 1, "changes": [{"tool", "path", "semantic": [...], "diff": "masked unified diff"}]}`.
- `status --exit-code`: exit code `3` when drift exists, `0` when synced.

### Central Config

Known fields (`name` / `npm` / `baseURL` / `apiKeyEnv` / `models`) are normalized, while tool-specific unknown fields are namespaced under `x.<tool>` and restored when pushing back to the same tool — nothing is lost across a pull/push round trip.

```json
{
  "providers": {
    "llm-01": {
      "apiKeyEnv": "LLM01_API_KEY",
      "baseURL": "https://llm-01.example.com/v1",
      "models": {
        "model-a": {
          "name": "Model A"
        },
        "model-b": {
          "name": "Model B"
        }
      },
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "x": {
        "kilocode": {
          "reasoning": true
        }
      }
    }
  },
  "version": 1
}
```

`version` is the central config format version (reserved for future migrations; currently `1`).

### Model aliases

`aliases` in the central config maps a common model name to per-tool model IDs; `push` converts each tool's required ID format automatically.

```json
{
  "aliases": {
    "sonnet": {
      "kilocode": "claude-sonnet-4-5",
      "opencode": "anthropic/claude-sonnet-4-5"
    }
  },
  "providers": {
    "llm-01": {
      "models": { "sonnet": { "name": "Sonnet" } }
    }
  }
}
```

- Conversion happens only before push rendering. `pull` never rewrites IDs (no information loss across round trips).
- Model names not in `aliases` pass through unchanged.
- An alias without a mapping for the target tool passes through with a warning. `--strict` errors out and writes nothing.
- There is no built-in mapping table. The user-defined `aliases` are the source of truth and survive `pull`.

### Backups and undo

- Just before every write, all affected files are backed up as one operation.
- Location: `~/.local/state/provsync/` — `index.json` (operation history) and `backups/<op-id>/` (SHA-256-verified file snapshots).
- The last 20 operations are retained; older ones are pruned automatically. `PROVSYNC_KEEP` changes the retention count (an integer of 1 or more).

```console
$ provsync undo --list
20261002T093045-8c2d  2026-10-02 09:30:45  push opencode
    /home/you/.config/opencode/opencode.json
20261002T093012-3fa1  2026-10-02 09:30:12  pull kilocode
    /home/you/.config/provsync/config.json
```

- `provsync undo` restores the last write; `provsync undo <id>` restores a specific operation. It applies directly, without `--write`.
- undo records the pre-restore state as a new operation and prints an ID to redo with (`やり直し: provsync undo <id>`).
- `provsync undo --prune --keep <n>` removes history, keeping the newest n operations (the removed count is printed).
- Writes made with `--no-backup` are recorded as marker operations. A subsequent `undo` warns that the latest write was made without a backup and that it restores the state before that write.
- Newly created central configs and state directories are created with 0600 / 0700 permissions. Existing file permissions are never changed. `status` suggests `chmod` when permissions are loose.
- Symlinks from dotfiles management are preserved. Writes go to the link target's real file; the link itself is never replaced by a regular file. Broken links error out before writing. `list` shows the link target as ` (symlink → target)`.
- The `--write` and `undo` restore sections are protected by an exclusive lock (flock) on the state directory. Concurrent runs wait, then fail with "別の provsync が実行中" after a 10-second timeout. Preview, `diff`, and `status` take no lock. A crashed process recovers automatically because the OS releases the lock.

### Secret Handling

- The central config holds only `apiKeyEnv` (an environment variable name). Actual keys are never mediated.
- On `pull`, secret-like fields (`apiKey` / `api_key` / `token` / `secret` / `password` / `accessToken` / `access_token`, including inside `options`) are dropped with a warning:

```console
警告: 秘密情報らしいフィールド "options.apiKey" を検出しました。秘密は仲介しないため取り込みません
```

- On `push`, secret fields that already exist in the target tool config are preserved as-is (never deleted, never leaked into the central config).
- `apiKeyEnv` is never rendered for opencode (its keys live in `auth.json`).
- `diff` output masks values of secret-like keys (`apiKey` / `token`, etc.) as `********` by default. The written file content is never masked (display only).
- To inspect actual values, pass `--show-secrets`. Values are shown with a warning at the top.
- `status` / `list` show key-name warnings only; values are never printed.
- See [SECURITY.md](SECURITY.md) for how to report a vulnerability.

### Re-serialization Caveat

Tool configs are fully re-serialized (2-space indent, alphabetically sorted keys). Original comments and formatting are lost, so diffs may include formatting churn.

### Development

```bash
make check    # fmt-check → vet → test
make build    # bin/provsync
make lint     # staticcheck
make vuln     # govulncheck
make test-race  # race-detector tests
```

Package layout: `main.go` (dispatch), `internal/cli` (subcommands), `internal/model` (canonical representation), `internal/adapter` (kilocode / opencode conversion), `internal/store` (central config), `internal/syncer` (merge), `internal/jsonc` (JSONC preprocessing), `internal/plan` (change plan and semantic diff), `internal/diff` (unified diff), `internal/backup` (backup and restore), `internal/fsutil` (atomic write, JSON formatting).

- [CHANGELOG.md](CHANGELOG.md)
- [Design document](docs/superpowers/specs/2026-10-02-provsync-multi-tool-sync-design.md)

## License / ライセンス

MIT — see [LICENSE](LICENSE).
