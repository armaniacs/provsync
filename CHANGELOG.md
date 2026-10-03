# Changelog

このプロジェクトの主な変更点を記録する。
形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に、バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従う。

## [Unreleased]

`provsync-tui` はベータ版として提供する。画面構成・キー操作・CLI との連携契約などの仕様は今後変更する可能性がある。

### Added

- `make install` で CLI(`provsync`)と TUI(`provsync-tui`)の両方をインストールできるようにした。
- TUI の確認画面に `--write` なし push のプレビュー(意味差分)を表示し、適用後の完了画面で `u` による直前操作の取り消しができるようにした。

### Changed

- CLI の日本語表示を「セントラル設定」の表記に統一した（single source of truth の概念は「セントラルカノニカル設定」）。終了コード・JSON 出力は不変。

### Removed

- 英語版 CHANGELOG(`CHANGELOG.en.md`)を廃止した。メンテナンスを継続できないため。以降はこの日本語版を正とする。ドキュメントサイトの英語版 CHANGELOG ページは廃止の案内に置き換えた。

## [0.3.1] - 2026-10-03

### Fixed

- `provsync-tui` が適用 Cmd の完了結果をイベントループ外でモデルに書き込む競合を解消。結果を Msg で Update に返す形にし、画面遷移（confirm→done→quit、confirm→list）をテストで固定した。外部挙動は不変。

### Changed

- CLI・TUI のメッセージの既定言語を日本語から英語に変更(**表示の既定が変わる破壊的変更**)。`ja` で始まるロケール(`PROVSYNC_LANG` / `LC_ALL` / `LC_MESSAGES` / `LANG` の優先順位)でのみ日本語になり、未対応・空のロケールは英語にフォールバックする。`Message.Error()` の非ローカライズ描画も英語になる。`PROVSYNC_LANG=ja` を設定すれば従来どおり日本語で使える。

### Added (CI)

- Makefile に `all`(CLI と TUI の両方をビルド)/ `tui-test`(TUI モジュールのテスト)/ `test-all`(コアと TUI の両モジュールのテスト)を追加。

## [0.3.0] - 2026-10-03

CLI・TUI のメッセージの日英対応と、ドキュメントサイトの追加。

### Added

- CLI のメッセージを日英対応にした（新規パッケージ `internal/i18n` のカタログ経由）。既定は日本語のまま。`PROVSYNC_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` の優先順位で言語を判定し、`en` で始まるロケールなら英語、未対応・空のロケールは日本語にフォールバックする。`provsync-tui` も同じカタログを共用し（`tui/go.mod` の replace 指令）、子プロセスと表示言語が混在しない。`status --json` のキー構造はロケール間で不変だが、`drift` 行と `warnings` の値はロケールに連動する。機械可読な drift は従来どおり `driftEntries` を使う。

- ドキュメントサイト（MkDocs Material + mkdocs-static-i18n）を追加。push to main で GitHub Actions が自動デプロイする（`https://armaniacs.github.io/provsync/`）。日本語を既定とし、言語切替で英語版を提供する。収録: ホーム / 使い方 / CLI リファレンス / バックアップと undo / セキュリティ / 開発者向け / CHANGELOG。`make docs-serve` でローカルプレビュー、依存は `requirements.txt` に固定。
- `provsync --version` / `provsync version` でバージョンを表示。版の決定順はビルド時の ldflags 注入値、`go install` のモジュール版、`dev` の順。`make build` は `git describe` の結果を ldflags で注入する。
- 引数なし実行・`--help` の使い方の先頭にバージョンを表示。
- 引数なし実行・`--help` に、セントラル設定・各ツール設定・バックアップ保存先のパスと存在有無(未作成)を表示。セントラル設定が未作成のときは作成手順を案内する。
- `status --json` に `driftEntries`(drift の構造化版)を additive 追加。`op` は固定集合 `not-in-tool` / `not-in-central` / `drift`。既存の `drift` 文字列とテキスト出力は不変。`provsync-tui` は driftEntries を優先使用し、旧 CLI の応答には文字列逆解析でフォールバックする。
- `diff` の出力で、秘密情報らしいキー(`apiKey` / `token` など)の値を既定で `********` にマスク。`--show-secrets` で実値を表示(警告付き)。書き込まれるファイルの内容はマスクされない。秘密キーの判定は pull と共通の仕組み(`internal/secret`)に集約。
- `init [tool]` コマンド。初回セットアップ用に `pull` と同じ Plan でセントラル設定を作る。既存のセントラル設定は上書きしない。ツール省略時は設定ファイルが存在するツールを検出し、候補が 1 つなら自動選択する。秘密が検出されたときは `apiKeyEnv` への移行手順を警告する。

### Added (CLI UX)

- サブコマンド別ヘルプ(`provsync <command> --help`)と `completion bash|zsh|fish` を追加。
- 終了コードを体系化: `0` 成功 / `1` 実行時エラー / `2` 使い方の誤り。使い方の誤りは `UsageError` 型で判別する。
- 通常出力は stdout、警告とフラグ解析エラーは stderr へ分離(`RunWith` で stdout / stderr を注入可能)。

### Added (backup)

- バックアップ保持数を `PROVSYNC_KEEP` 環境変数で設定可能に(既定は 20 のまま)。
- `provsync undo --prune --keep <n>` で履歴を掃除。新しい n 件を残して削除し、削除件数を表示する。
- 新規作成されるファイルは 0600、新規ディレクトリは 0700 で作る(既存ファイルの権限は引き継ぐ)。`status` はセントラル設定・状態ディレクトリの権限が緩い場合に `chmod` を案内する。

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

### Added (aliases)

- セントラル設定の `aliases`(共通モデル名 → ツール名 → モデル ID)で、`push` 時にモデル名をツール向け ID へ自動変換。当該ツール向けの対応が未定義のエイリアスは警告して素通し、`--strict` でエラーにする。pull は ID を書き換えず、`aliases` は pull しても消えない。スキーマは追加のみで version は 1 のまま。

### Added (tests)

- `FuzzStripJSONC`(標準の `testing.F`)を追加。文字列リテラル内のコメント記号や末尾カンマを壊さないことをファズで検証。`make fuzz` で短時間実行できる。
- 全アダプタの pull → push 往復の冪等性テストと、push 出力のゴールデンテスト(`-args -update` で更新)を追加。

### Added (lock)

- `--write` の書き込み区間と `undo` の復元区間に、状態ディレクトリの排他ロック(`syscall.Flock`)を導入。後発のプロセスは待機し、タイムアウト(10 秒)でエラー終了。プレビュー・`diff`・`status` はロックを取らない。PID ファイル方式は使わず、プロセス終了で OS が自動解放する。

### Added (doctor)

- `provsync doctor` コマンド。パス解決・セントラル設定と各ツール設定の存在と構文・`apiKeyEnv` の環境変数の設定有無・ファイル権限を一覧で診断する。通信せずファイルも書かない。NG があるときは終了コード 1。秘密の値は出力しない。

### Added (docs)

- CONTRIBUTING.md(貢献者向け)を追加。開発手順・コミット規約・秘密の扱いの原則・新しいツールのアダプタ追加ガイドを記載。
- 全アダプタに適用する共通の契約テスト(`TestAdapterContract`)を追加。往復冪等、ファイル内の秘密キー保持、未知フィールド保持、秘密がカノニカル形へ漏れないことを検証する。

### Added (check)

- `provsync check` コマンド。セントラル設定の各 provider について `baseURL` の `/models` へ認証付き GET を送り到達可否を分類表示する(`[OK]` / `[認証失敗]` / `[応答異常]` / `[到達不可]` / `[スキップ]`)。タイムアウト 5 秒。通信するのはこのコマンドだけで、キーの値は出力しない。

### Added (routes)

- セントラル設定の `routes`(エイリアス名 → provider キーの優先順)で、`push` 時に使える経路(apiKeyEnv 設定済みまたは不要)を選び、選ばれなかった経路の provider を描画対象から除く。判定は環境変数の有無のみで通信しない。どの経路も使えないときは警告して変更しない。`routes` は pull しても消えない。スキーマは追加のみで version は 1 のまま。

### Added (tui)

- `provsync-tui`(任意)。ターミナル上で provider と同期先ツールをチェックボックスで選び、確認画面の承認後に適用する TUI。`tui/` に独立した Go モジュールとして隔離し、コアの `go.mod` に外部依存を追加しない。TUI は `provsync` バイナリを子プロセスとして呼び、変更内容を自前で計算しない。非対話端末ではエラー終了する。依存の隔離方式の決定記録は `docs/superpowers/specs/2026-10-03-tui-isolation-decision.md`。なお、この時点では参考実装の位置づけであり、実用に耐える水準には達していない。ベータ版として提供し、仕様は今後変更する。

### Added (CI)

- CI(`.github/workflows/ci.yml`)を追加。push to main と PR で `make check` / `make test-race` / `make lint` / `make vuln` を macOS / Linux マトリクスで実行。
- Makefile に `lint`(staticcheck)/ `vuln`(govulncheck)/ `test-race` を追加。lint ツールは `go run pkg@version` で取得し、`go.mod` に依存を追加しない。

### Docs

- README: `diff` 出力例を実挙動(秘密マスク)に合わせて更新。`--show-secrets` フラグを追加。
- SECURITY.md(セキュリティポリシー)を追加。脆弱性の非公開報告の方法と対象範囲を記載。

## [0.2.1] - 2026-10-02

軽微な修正とドキュメント整備。

### Fixed

- 空の `models` を持つ provider が、pull 後も `diff` のプレビューで「更新 (models)」、`status` で「差分あり」と表示され続ける問題。セントラル設定への保存で空の `models` が省略されることによる nil と空オブジェクトの比較差で、ファイルは変わらない偽差分だった。
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
- セントラルカノニカル設定 `~/.config/provsync/config.json` を唯一の正とする設計へ変更。
- provider の既知フィールドを正規化し、ツール固有の未知フィールドは `Extras` に保持して往復。
- `--provider` はカンマ区切り・繰り返し指定に対応。バックアップ無効化は `--no-backup` に変更。
- 意味差分をツール可視の射影(adapter の `Project`)で比較し、ツールが描画しないフィールド(`apiKeyEnv` 等)による `status`・プレビューの誤検知を解消。
- pull は既存のセントラル設定から他ツールの extras 名前空間と `version` を保持する。
- `status` は壊れたセントラル設定をエラー扱いにする(未作成とは区別)。

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