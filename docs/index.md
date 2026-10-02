# provsync

**複数の LLM ツールの provider 設定を、たった一つの中央設定に同期する**

provsync は、複数の LLM コーディングツールがそれぞれ独自の形式で持つ `provider` 設定を、中央のカノニカル設定を唯一の正(single source of truth)として同期する CLI ツール。`pull` でツールから中央へ取り込み、`push` で中央からツールへ反映する。Go 標準ライブラリのみで実装され、外部依存はない。

## 安全設計

既定の動作はプレビューのみで、`--write` を付けたときにだけファイルが変わる。書き込みの直前には、影響する全ファイルがタイムスタンプ付きの 1 操作として自動でバックアップされる。取り消しも `undo` ででき、undo 自体が復元前の現状を新しい操作として記録するので、取り消しの取り消し(redo)もできる。

見どころは Plan の一点だ。プレビュー・`diff`・適用・バックアップは、すべて同一の変更計画(Plan)から生成される。そのため「diff で見た内容」がそのまま「書かれる内容」であり、undo で戻る内容とも一致する。同じ状態への再書き込みはスキップされる(冪等)ので、変更が無ければ `変更はありません` と表示されるだけだ。

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

Go 1.25.14 以上。

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

このドキュメントで解決しない疑問は、[GitHub の Issues](https://github.com/armaniacs/provsync/issues) から報告してください。不具合ではなく改善の提案は、[CONTRIBUTING](https://github.com/armaniacs/provsync/blob/main/CONTRIBUTING.md) の手順に沿って受け付けます。
