# TUI の依存隔離方式（決定記録）

## 決定

TUI ダッシュボード（`provsync-tui`）は、コアリポジトリの `tui/` ディレクトリに
**独立した Go モジュール**（`module github.com/armaniacs/provsync/tui`）として実装する。
コアの `go.mod` には外部依存を追加しない。TUI の外部依存（Bubbletea）は
`tui/go.mod` のみに置く。

## 根拠

- コア CLI は Go 標準ライブラリのみで実装する方針を維持する。TUI の端末制御を
  標準ライブラリだけで実装するのは重く、Bubbletea のような外部依存が現実的。
- 独立モジュールに隔離することで、TUI を使わない利用者・貢献者は依存の影響を
  受けない。導入は利用者の任意。

## 方式

- TUI はコアの `internal` を import できない（別モジュールのため）。
- TUI は `provsync` バイナリを**子プロセスとして呼ぶ**方式にする:
  - 状態取得: `provsync status --json`（`schemaVersion: 1` のスキーマ）
  - 適用: `provsync push <tool> --provider <p> --write`
- TUI は選択状態を集めて確認画面でプレビューを見せ、承認されたときだけ
  `--write` 付きで子プロセスを実行する。変更内容を自前で計算しない
  （変更計画の単一情報源はコアの `plan.Plan`）。
- 標準入力が端末でないときは「対話が必要です」でエラー終了する。

## 運用

- ビルド・テストは `tui/` 内で `go build ./...` / `go test ./...` を実行する。
- CI（`.github/workflows/ci.yml`）はコアモジュールのみを対象とし、tui モジュールは含めない。
