# Changelog

このプロジェクトの主な変更点を記録する。
形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に、バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従う。

## [Unreleased]

### Added

- `provsync --version` / `provsync version` でバージョンを表示。版の決定順はビルド時の ldflags 注入値、`go install` のモジュール版、`dev` の順。`make build` は `git describe` の結果を ldflags で注入する。
- 引数なし実行・`--help` の使い方の先頭にバージョンを表示。
- 引数なし実行・`--help` に、中央設定・各ツール設定・バックアップ保存先のパスと存在有無(未作成)を表示。中央設定が未作成のときは作成手順を案内する。
- `diff` の出力で、秘密情報らしいキー(`apiKey` / `token` など)の値を既定で `********` にマスク。`--show-secrets` で実値を表示(警告付き)。書き込まれるファイルの内容はマスクされない。秘密キーの判定は pull と共通の仕組み(`internal/secret`)に集約。
- `init [tool]` コマンド。初回セットアップ用に `pull` と同じ Plan で中央設定を作る。既存の中央設定は上書きしない。ツール省略時は設定ファイルが存在するツールを検出し、候補が 1 つなら自動選択する。秘密が検出されたときは `apiKeyEnv` への移行手順を警告する。

### Added (CLI UX)

- サブコマンド別ヘルプ(`provsync <command> --help`)と `completion bash|zsh|fish` を追加。
- 終了コードを体系化: `0` 成功 / `1` 実行時エラー / `2` 使い方の誤り。使い方の誤りは `UsageError` 型で判別する。
- 通常出力は stdout、警告とフラグ解析エラーは stderr へ分離(`RunWith` で stdout / stderr を注入可能)。

### Added (backup)

- バックアップ保持数を `PROVSYNC_KEEP` 環境変数で設定可能に(既定は 20 のまま)。
- `provsync undo --prune --keep <n>` で履歴を掃除。新しい n 件を残して削除し、削除件数を表示する。
- 新規作成されるファイルは 0600、新規ディレクトリは 0700 で作る(既存ファイルの権限は引き継ぐ)。`status` は中央設定・状態ディレクトリの権限が緩い場合に `chmod` を案内する。

### Added (release)

- リリース自動化(`.goreleaser.yaml` と `.github/workflows/release.yml`)。タグ push で darwin / linux(amd64・arm64)の tar.gz アーカイブと checksums を GitHub Release に添付する。CHANGELOG に該当バージョンの節が無い場合は失敗する。
- README のインストール手順にバイナリ入手の方法を追記。対応環境に macOS / Linux(Windows は非対応)を明記。

### Fixed

- 書き込み先がシンボリックリンクの場合、リンク自体が通常ファイルに置き換わる問題。リンクを維持したままリンク先の実体を atomic に更新する。リンク切れは明確なエラーにする。`list` はリンク先を表示する。

### Added (platform)

- Windows は非対応であることを起動時に明示(`cli.Supported`。Windows では終了コード 1)。
- `adapter.NewRoot` の XDG 解決(`XDG_CONFIG_HOME` / `XDG_STATE_HOME`)の回帰テストを追加。

### Added (JSON)

- `list` / `status` / `diff` に `--json` を追加(`schemaVersion: 1` 含む)。`--json` 時は標準出力が JSON のみになり、警告は stderr へ。
- `status` に `--exit-code` を追加。差分があるとき終了コード 3 で終了する(`ExitError` 型で判別)。

### Added (CI)

- CI(`.github/workflows/ci.yml`)を追加。push to main と PR で `make check` / `make test-race` / `make lint` / `make vuln` を macOS / Linux マトリクスで実行。
- Makefile に `lint`(staticcheck)/ `vuln`(govulncheck)/ `test-race` を追加。lint ツールは `go run pkg@version` で取得し、`go.mod` に依存を追加しない。

### Docs

- README: `diff` 出力例を実挙動(秘密マスク)に合わせて更新。`--show-secrets` フラグを追加。
- SECURITY.md(セキュリティポリシー)を追加。脆弱性の非公開報告の方法と対象範囲を記載。

## [0.2.1] - 2026-10-02

軽微な修正とドキュメント整備。

### Fixed

- 空の `models` を持つ provider が、pull 後も `diff` のプレビューで「更新 (models)」、`status` で「差分あり」と表示され続ける問題。中央設定への保存で空の `models` が省略されることによる nil と空オブジェクトの比較差で、ファイルは変わらない偽差分だった。
- `diff` のヘッダが絶対パスのとき `a//home/...` とスラッシュが重なって表示される問題。

### Docs

- README: 出力例を実挙動に合わせて修正(`status` の秘密検出の警告行、`diff` ヘッダの表記)。秘密情報の扱いに `diff`・`status` での秘密の表示に関する注意を追記。`version` フィールドと `--help` の説明、必要 Go バージョン(1.25.14)を明記。
- AGENTS.md: 削除済みフラグ一覧の誤り(`--write` → `--backup`)を修正。
- 設計書(2026-10-02)を as-built として更新(マーカー操作、undo 警告、`models` の等価扱いなど)。
- 旧設計・実装計画(2026-10-01)に「置き換え済み」の注記を追加。

## [0.2.0] - 2026-10-02

マルチツール同期への再設計。

### Changed

- CLI をサブコマンド化(`list` / `status` / `pull` / `push` / `sync` / `diff` / `undo`)。旧フラグ(`--source` / `--target` / `--providers` / `--backup`)を削除(**破壊的変更**)。
- 中央カノニカル設定 `~/.config/provsync/config.json` を唯一の正とする設計へ変更。
- provider の既知フィールドを正規化し、ツール固有の未知フィールドは `Extras` に保持して往復。
- `--provider` はカンマ区切り・繰り返し指定に対応。バックアップ無効化は `--no-backup` に変更。
- 意味差分をツール可視の射影(adapter の `Project`)で比較し、ツールが描画しないフィールド(`apiKeyEnv` 等)による `status`・プレビューの誤検知を解消。
- pull は既存の中央設定から他ツールの extras 名前空間と `version` を保持する。
- `status` は壊れた中央設定をエラー扱いにする(未作成とは区別)。

### Added

- `Plan` を単一情報源とするプレビュー / diff / 適用。
- 意味差分 + 統合 diff の表示(`diff`)。
- タイムスタンプ付きバックアップ、マニフェスト、`undo`(undo 自体を undo できる)。undo はやり直し用の操作 ID を表示する。
- アダプタ IF(kilocode / opencode、別名 `kilo`)。
- 秘密は仲介しない方針(`apiKeyEnv` 参照のみ)。push は対象ツール設定内の既存の秘密フィールド(`options.apiKey` 等)を保持する。
- `--no-backup` の書き込みはマーカー操作として履歴に記録され、後続の `undo` で警告される。

### Removed

- 単発の `kilo.jsonc` → `opencode.json` 移植 CLI と旧 `main_test.go`。

### 移行ガイド(0.1.0 から)

- 旧 `provsync --source X --target Y --write` 相当は `provsync sync --from kilocode --to opencode --write`。
- 旧 `--providers a,b` 相当は `--provider a --provider b`(カンマ区切りも可)。
- 旧 `--backup=false` 相当は `--no-backup`。付けない場合、書き込み前に自動でバックアップされる。
- 0.1.0 が作成した `opencode.json.bak` は新しい `undo` 履歴には含まれない(手動での復元のみ)。

## [0.1.0] - 2026-10-01

初回リリース。

### Added

- `provsync` CLI。`~/.config/kilo/kilo.jsonc` の provider エントリを `~/.config/opencode/opencode.json` へ移植する。
- JSONC(行コメント・末尾カンマ)の前処理 `StripJSONC`。
- provider を丸ごと上書き移植する `Merge`(対象外キーは保持)。
- 既定はプレビュー。`--write` でファイルを更新。
- 書き込み時のバックアップ(`.bak`)とアトミック置換。
- フラグ: `--source` / `--target` / `--write` / `--backup` / `--providers`。
- 単体テストおよび `run()` の統合テスト。
- `Makefile`(build / test / check / write など)。
- `README.md`。