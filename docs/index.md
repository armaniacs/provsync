# provsync

**複数の LLM ツールの provider 設定を、たった一つの中央設定に同期する**

provsync は、複数の LLM コーディングツールがそれぞれ独自の形式で持つ `provider` 設定を、中央のカノニカル設定を唯一の正(single source of truth)として同期する CLI ツール。`pull` でツールから中央へ取り込み、`push` で中央からツールへ反映する。Go 標準ライブラリのみで実装され、外部依存はない。

## 安全設計

- 既定はプレビューのみ。`--write` を付けたときだけファイルが変わる。
- 書き込みの直前に、影響する全ファイルをタイムスタンプ付きの 1 操作として自動バックアップ。
- `undo` は復元前の現状も新しい操作として記録するため、undo 自体を undo できる(redo 可能)。
- プレビュー・`diff`・適用・バックアップはすべて同一の変更計画(Plan)から生成される。「diff で見た内容」=「書かれる内容」=「undo で戻る内容」。
- バイト単位で同一の書き込みはスキップされる(冪等)。変更が無ければ `変更はありません` と表示されるだけ。

## 対応ツール

macOS / Linux 対応。Windows は非対応。

| ツール | 設定ファイル | 形式 |
|---|---|---|
| kilocode(別名 `kilo`) | `~/.config/kilo/kilo.jsonc` | JSONC(行コメント・末尾カンマ対応) |
| opencode | `~/.config/opencode/opencode.json` | JSON |

- 中央設定: `~/.config/provsync/config.json`
- 状態(バックアップと履歴): `~/.local/state/provsync/`
- パスは `XDG_CONFIG_HOME` / `XDG_STATE_HOME` を尊重。

## インストール

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

## はじめに

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

次のステップ:

- [使い方](usage.md) — クイックスタートと機能の詳細
- [CLI リファレンス](cli.md) — コマンド・フラグ・終了コードの一覧
- [バックアップと undo](backup.md) — 復元と履歴の管理
