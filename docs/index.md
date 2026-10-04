---
title: provsync
---

<div class="tx-hero" markdown>

<div class="tx-hero__content" markdown>

# provsync

**複数の LLM ツールの provider 設定を、ただ一つのセントラル設定に同期する**

`pull` でツールから中央へ取り込み、`push` で中央からツールへ反映する。書き込みの前には自動バックアップ、取り消しは `undo` で。

[使い方をみる](usage.md){ .md-button .md-button--primary }
[GitHub で開く](https://github.com/armaniacs/provsync){ .md-button }

</div>

</div>

<div class="grid cards" markdown>

- :material-shield-check-outline:{ .lg .middle } __安全に書き換える__

    ---

    プレビューが既定。書き込みの直前には影響する全ファイルが自動でバックアップされ、`undo` と redo で元に戻せる

- :material-database-outline:{ .lg .middle } __セントラル設定を唯一の正に__

    ---

    pull → push の往復で情報は失われない。`aliases` でモデル ID を各ツール向けに自動変換

- :material-stethoscope:{ .lg .middle } __壊れる前に気づく__

    ---

    `doctor` で環境を診断し、`check` で API の到達可否を確認する

</div>

## 対応ツール

macOS / Linux 対応。Windows は非対応。

| ツール | 設定ファイル | 形式 |
|---|---|---|
| kilocode(別名 `kilo`) | `~/.config/kilo/kilo.json[c]` | JSONC(行コメント・末尾カンマ対応) |
| opencode | `~/.config/opencode/opencode.json[c]` | JSON / JSONC(どちらも JSONC として読む) |

- 複数候補があるときは `.jsonc` を優先し、先に見つかった1ファイルだけを管理する(ツール本体のような deep-merge はしない)。
- セントラル設定: `~/.config/provsync/config.json`
- 状態(バックアップと履歴): `~/.local/state/provsync/`
- パスは `XDG_CONFIG_HOME` / `XDG_STATE_HOME` を尊重。

## インストール

Go 1.25.14 以上。

```bash
# 最新版
go install github.com/armaniacs/provsync@latest

# バージョン指定
go install github.com/armaniacs/provsync@v0.3.3
```

パッケージの詳細は [pkg.go.dev](https://pkg.go.dev/github.com/armaniacs/provsync) を参照。

Go を使わない場合は、Releases からビルド済みバイナリを入手する。

```bash
# macOS (Apple Silicon) の例
VER=0.3.3
curl -sLO https://github.com/armaniacs/provsync/releases/download/v${VER}/provsync_${VER}_darwin_arm64.tar.gz
tar xzf provsync_${VER}_darwin_arm64.tar.gz
sudo mv provsync /usr/local/bin/
```

ソースからビルドする場合:

```bash
git clone https://github.com/armaniacs/provsync.git
cd provsync
make build    # bin/provsync
```

## はじめに

初めて使うときは `provsync init` でセントラル設定を作る。

```console
# 取り込み元のツールを指定(まずはプレビュー)
$ provsync init kilocode

# --write でセントラル設定を作成
$ provsync init kilocode --write

# 同期状態を確認して、他のツールへ反映する
$ provsync status
$ provsync push opencode --write
```

ツールを省略すると、設定ファイルが存在するツールを検出する。候補が複数ある場合は `provsync init <tool>` で指定する。セントラル設定が既にある場合は上書きしないため、日常の更新には `pull` を使う。

## 次のステップ

- [使い方](usage.md) — クイックスタートと機能の詳細
- [CLI リファレンス](cli.md) — コマンド・フラグ・終了コードの一覧
- [バックアップと undo](backup.md) — 復元と履歴の管理

このドキュメントで解決しない疑問は、[GitHub の Issues](https://github.com/armaniacs/provsync/issues) から報告してください。不具合ではなく改善の提案は、[CONTRIBUTING](https://github.com/armaniacs/provsync/blob/main/CONTRIBUTING.md) の手順に沿って受け付けます。
