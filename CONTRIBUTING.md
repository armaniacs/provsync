# CONTRIBUTING

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

- 秘密の値(`apiKey` / `token` など)を出力・ログ・エラーメッセージ・テストの期待値に出さない(テスト用のダミー文字列 `sk-test-...` を除く)。
- 秘密の判定は `internal/secret` に集約する。pull と出力層で共有し、二重に判定を持たない。
- 中央設定は `apiKeyEnv`(環境変数名)のみを保持し、値は中継しない。
- ファイル書き込みは `fsutil.WriteFileAtomic` を通す(一時ファイル + rename)。
- 「何が変わるか」を `internal/cli` で別計算しない。`plan.Plan` を作って再利用する。

## 新しいツールのアダプタを追加する手順

provsync の価値は対応ツールの広がりにある。新しいツールは次の手順で足す。

1. `internal/adapter/<tool>.go` を作り、`Adapter` インターフェイス(`Name` / `Path` / `Pull` / `Push` / `Project`)を実装する。共通処理(`pullDocument` / `pushDocument` / `projectProviders`)を呼ぶだけの薄い層にする。
   - `Pull` は秘密情報らしいキー(`internal/secret` の判定)を警告のうえ取り込まないこと。共通部が処理する。
   - `Push` は managed 外の provider と provider 以外の設定を保持すること。共通部が処理する。
   - ツールがファイル内の秘密キーを保持している場合、`carryOverSecrets` が同一ツールへの push で値を運ぶ。
2. `adapter.Get` の `switch` と `Names()` に追加する。
3. 契約テスト(`internal/adapter/contract_test.go`)が自動で走ることを確認する。契約:
   - pull → push の出力は冪等
   - ファイル内の秘密キーの値は保持される
   - ツール固有の未知フィールドは失われない
   - 秘密の値がカノニカル形へ漏れ出ない
4. CLI のヘルプ・補完に反映する必要がある場合は `internal/cli/cli.go` の `helpTexts` と README の「対応ツール」を更新する。

## アダプタが満たすべき振る舞い(契約テストの対象)

| 振る舞い | 確認するテスト |
|---|---|
| pull → push が冪等 | `TestAdapterContract` / `TestPushIsIdempotentPerTool` |
| ファイル内の秘密キーを保持 | `TestAdapterContract` |
| 未知フィールドの保持 | `TestAdapterContract` / `TestPullPushRoundTripsUnknownFields` |
| 秘密がカノニカル形へ漏れない | `TestAdapterContract` |
| push 出力の安定性 | `TestPushGolden` |
