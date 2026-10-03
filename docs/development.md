# 開発者向け

provsync への貢献の手順。

## 開発の始め方

Go 1.25.14 以上が必要。

```bash
git clone https://github.com/armaniacs/provsync.git
cd provsync
make check    # fmt-check → vet → test。終了前に必ず通す
make build    # bin/provsync
make lint     # staticcheck
make vuln     # govulncheck
make test-race
make fuzz     # StripJSONC の短時間ファズテスト
```

`make check` が通らない変更はマージしない。CI も同じコマンドを実行する(`.github/workflows/ci.yml`)。

## コミット規約

- コミットメッセージは英語の Conventional Commits(`feat:` / `fix:` / `docs:` / `test:` / `chore:`)。
- ドキュメント・CLI メッセージ・コード内コメントは日本語。識別子は英語。
- コメントは「なぜ」だけを書く(「何をしているか」は書かない。変更履歴・issue 番号も書かない)。
- `git add -A` / `git add .` は使わず、対象ファイルを個別に指定する。

## CHANGELOG

ユーザーに見える変更は `CHANGELOG.md` の `[Unreleased]` 節に追記する。形式は Keep a Changelog / SemVer。

## 秘密の扱いの原則

秘密の値(`apiKey` / `token` など)を出力・ログ・エラーメッセージ・テストの期待値に書かない。テスト用のダミー文字列(`sk-test-...`)はこの限りでない。

秘密の判定は `internal/secret` の 1 箇所に集約し、pull と出力層の両方がそこを参照する。判定を二重に持つと片方だけ更新漏れが起きるためだ。セントラル設定は `apiKeyEnv`(環境変数名)のみを保持し、値は中継しない。

ファイルの書き込みは `fsutil.WriteFileAtomic` を通す(一時ファイル + rename)。「何が変わるか」を `internal/cli` で別計算せず、`plan.Plan` を作って再利用する。

## 新しいツールのアダプタを追加する手順

provsync の価値は対応ツールの広がりにある。新しいツールは次の手順で足す。

1. `internal/adapter/<tool>.go` を作り、`Adapter` インターフェイス(`Name` / `Path` / `Pull` / `Push` / `Project`)を実装する。共通処理(`pullDocument` / `pushDocument` / `projectProviders`)を呼ぶだけの薄い層にする。
2. `adapter.Get` の `switch` と `Names()` に追加する。
3. 契約テスト(`internal/adapter/contract_test.go`)が自動で走ることを確認する。契約: pull → push の冪等、ファイル内の秘密キーの保持、未知フィールドの保持、秘密がカノニカル形へ漏れないこと。
4. CLI のヘルプ・補完に反映する必要がある場合は `internal/cli/help.go` のレジストリと README の「対応ツール」を更新する。

## パッケージ構成

- `main.go` — 起動のみ。`cli.Supported` で OS 判定し、`cli.RunWith` がテスト可能なエントリポイント
- `internal/cli` — サブコマンド(`cli.go`(ディスパッチと共通書き込み)、`sync.go`(同期系と前処理)、`status.go`(一覧・状態)、`history.go`(diff・undo)、`help.go`(コマンドレジストリ・ヘルプ・補完)、`doctor.go`(診断)、`check.go`(疎通確認)、`errors.go`(使い方エラー)、`render.go`(共通描画)、`platform.go`(対応 OS)、`version.go`(バージョン表示))
- `internal/model` — カノニカル表現
- `internal/adapter` — kilocode / opencode 変換
- `internal/store` — セントラル設定
- `internal/syncer` — マージ・エイリアス・経路選択
- `internal/jsonc` — JSONC 前処理
- `internal/plan` — 変更計画と意味差分
- `internal/diff` — 統合 diff
- `internal/backup` — バックアップと復元
- `internal/lock` — flock 排他ロック
- `internal/secret` — 秘密キー判定とマスク
- `internal/fsutil` — atomic write・JSON 整形
- `internal/version` — バージョン文字列解決
- `tui/` — bubbletea TUI(独立モジュール。`tui/go.mod` が replace で親を参照)

## ドキュメントサイトの更新

このサイトは MkDocs Material で生成され、push to main で自動デプロイされる。

```bash
make docs-serve    # ローカルプレビュー(http://localhost:8000)
make docs          # site/ へビルド
```

依存は `requirements.txt` に固定されている(`pip install -r requirements.txt`)。

## 関連ドキュメント

- [設計ドキュメント](https://github.com/armaniacs/provsync/blob/main/docs/superpowers/specs/2026-10-02-provsync-multi-tool-sync-design.md)
- [CONTRIBUTING.md](https://github.com/armaniacs/provsync/blob/main/CONTRIBUTING.md)(貢献者向け: アダプタ追加ガイド)
