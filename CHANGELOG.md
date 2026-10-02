# Changelog

このプロジェクトの主な変更点を記録する。
形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に、バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従う。

## [Unreleased]

### Added

- `provsync --version` / `provsync version` でバージョンを表示。版の決定順はビルド時の ldflags 注入値、`go install` のモジュール版、`dev` の順。`make build` は `git describe` の結果を ldflags で注入する。
- 引数なし実行・`--help` の使い方の先頭にバージョンを表示。
- 引数なし実行・`--help` に、中央設定・各ツール設定・バックアップ保存先のパスと存在有無(未作成)を表示。中央設定が未作成のときは作成手順を案内する。

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