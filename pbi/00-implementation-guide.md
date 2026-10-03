# 実装共通ガイド（全 PBI 共通・最初に読む）

各 PBI の末尾にある「実装ガイド」は、このガイドの規則を前提にしている。迷ったら推測せず、この文書と PBI の記述に戻る。

## 1. 進め方の型（必ずこの順）

1. PBI のファイルを最初から最後まで読む（BDD シナリオと受け入れ基準が完了条件）。
2. 実装ガイドに書かれた「先に読むファイル」を読む。読まずに書き始めない。
3. テストを先に書く。`go test ./... ` で失敗することを確認する（失敗しない新規テストは、テストが間違っている）。
4. 最小限の実装でテストを通す。
5. `make check` を実行し、全部通るまで直す（fmt-check → vet → test の順）。
6. README / CHANGELOG（`[Unreleased]` 節）を更新する。
7. コミットする（後述）。1 PBI = 1〜数個のコミット。複数 PBI を混ぜない。

## 2. 絶対に守る規則（破ると差し戻し）

- 外部依存を追加しない。`go.mod` に `require` を書かない。標準ライブラリのみ。
- 「何が変わるか」を `internal/cli` で別計算しない。必ず `plan.Plan` を作って `applyOrPreview` に渡す。
- ファイル書き込みは `fsutil.WriteFileAtomic` を通す（一時ファイル + rename）。`os.WriteFile` で設定ファイルを書かない。
- 既定はプレビュー。`--write` が無いときにファイルを変更しない。
- 秘密（`apiKey`, `token` など）の値を、出力・ログ・中央設定・エラーメッセージに出さない。
- ユーザー向けメッセージは `internal/i18n` のカタログ経由で出す。ja が正、en を mirror し、新規の文言は両方のカタログに追加する。コメント・ドキュメントは日本語。識別子は英語。
- コメントは「なぜ」だけを書く（「何をしているか」は書かない。変更履歴・PBI 番号も書かない）。
- `git add -A` / `git add .` を使わない。ファイルを個別に指定する。
- コミットメッセージは英語の Conventional Commits（`feat:` `fix:` `docs:` `test:` `chore:`）。
- push・タグ作成・GitHub 上の設定変更は、ユーザーの許可なしに行わない。

## 3. コードの地図

| 場所 | 役割 | 主な関数・型 |
|---|---|---|
| `main.go` | 起動のみ。`cli.Supported` で OS 判定し、`cli.RunWith(os.Args[1:], os.Stdout, os.Stderr)` を呼びエラーなら終了コード 1（使い方の誤りは 2） | |
| `internal/cli/cli.go` | フラグ解析・共通 option・適用（Plan の書き出し） | `Run`, `RunWith`, `options`, `registerFlags`, `reorder`, `applyOrPreview`, `newBackupStore` |
| `internal/cli/sync.go` | init / pull / push / sync | `cmdInit`, `cmdPull`, `cmdPush`, `cmdSync`, `pullFromTool`, `toolChangeForPush`, `buildCentralChange`, `buildToolChange`, `buildSyncPlan` |
| `internal/cli/status.go` | list / status | `cmdList`, `cmdStatus`, `buildStatusReport`, `driftEntries` |
| `internal/cli/history.go` | diff / undo | `cmdDiff`, `cmdUndo` |
| `internal/cli/help.go` | コマンドレジストリ・ヘルプ・補完 | `commandRegistry`, `lookupCommand`, `printUsage`, `cmdVersion`, `cmdCompletion` |
| `internal/cli/doctor.go` | doctor 診断 | `cmdDoctor`, `doctorCheck*` |
| `internal/cli/check.go` | check 疎通確認（唯一の通信経路） | `cmdCheck`, `checkProvider` |
| `internal/cli/errors.go` | エラー型と使い方エラー | `UsageError`, `ExitError`, `usageErr` |
| `internal/cli/render.go` | 出力の描画 | `renderPreview`, `renderSemantic`, `encodeJSON`, `warnLoosePerm` |
| `internal/cli/platform.go` | 対応 OS の判定 | `Supported` |
| `internal/adapter/adapter.go` | ツール設定の読み書き共通部 | `Root`, `NewRoot`, `ResolveRoot`, `Adapter` IF, `Names`, `Get`, `decodeProvider`, `encodeProvider`, `pullDocument`, `pushDocument`, `carryOverSecrets` |
| `internal/adapter/kilocode.go` `opencode.go` | ツール別の薄い層 | `Pull` / `Push` / `Project` |
| `internal/model/model.go` | 中央設定の型 | `Config`, `Provider`（`Models` は `map[string]any`）, `Version` |
| `internal/store/store.go` | 中央設定の読み書き | `Load`, `Marshal` |
| `internal/syncer/syncer.go` | provider 集合のマージ・絞り込み（純粋関数） | `MergeToolProviders`, `FilterProviders` |
| `internal/syncer/aliases.go` | モデルエイリアスの解決 | `ResolveAliases` |
| `internal/syncer/routes.go` | 経路の選択 | `SelectRoute` |
| `internal/plan/plan.go` | 変更計画 | `Plan`, `FileChange`, `ProviderChange`, `ProvidersDiff`, `ChangedFields` |
| `internal/diff/diff.go` | unified diff | `Unified(path, before, after, context)` |
| `internal/backup/backup.go` | バックアップ・undo | `Store`, `Record`, `RecordMarker`, `Restore`, `List`, `Find`, `Prune`, `MaxOperations` |
| `internal/lock/lock.go` | 状態ディレクトリの flock 排他 | `Acquire` |
| `internal/secret/secret.go` | 秘密キー判定とマスク | `IsKey`, `MaskLines` |
| `internal/fsutil/fsutil.go` | atomic 書き込み・ソート JSON | `WriteFileAtomic`, `MarshalIndentSorted` |
| `internal/i18n/` | 言語判定と ja/en メッセージカタログ | `Resolve`, `ResolveFromEnv`, `T`, `Message`, `Localize` |
| `internal/jsonc/jsonc.go` | JSONC → JSON | `StripJSONC`（行コメントと末尾カンマのみ。ブロックコメントは非対応） |
| `internal/version/version.go` | バージョン文字列の解決 | `String` |
| `tui/` | bubbletea TUI（独立モジュール。`tui/go.mod` が replace で親を参照し `status --json` をパース） | `listModel`, `selection`, `fetchStatus` |

設定ファイルの実パス: kilocode は `<ConfigHome>/kilo/kilo.jsonc`、opencode は `<ConfigHome>/opencode/opencode.json`、中央設定は `<ConfigHome>/provsync/config.json`、バックアップは `<StateHome>/provsync/`。`ConfigHome` は `XDG_CONFIG_HOME` があればそれ、なければ `~/.config`。

## 4. cli のテストの書き方

`internal/cli/cli_test.go` に補助関数がある。新しいテストは必ずこれを使う。

- `setup(t)` : 一時ディレクトリに kilocode / opencode の設定を作り、`fixture{root, kilo, opencode, central}` を返す。中央設定はまだ無い。
- `mustRun(t, f.root, "pull", "kilocode", "--write")` : `--root` 付きで実行し、エラーならテスト失敗。出力文字列を返す。
- `run(t, f.root, ...)` : エラーを自分で検査したいときに使う（戻り値 `(out, err)`）。
- `read(t, path)` : ファイルを文字列で読む。 `write(t, path, content)` : ファイルを書く。
- 実ホームを触らない。`t.TempDir()` と `--root` だけを使う。環境変数は `t.Setenv`。

テスト名は `TestXxx`。BDD シナリオ 1 つにつきテスト 1 つ以上。

## 5. 新しいサブコマンドを足す手順

1. `internal/cli/help.go` の `commandRegistry()` に 1 エントリ足す（`name` / `usage` / `summaryKey` / `helpKey` / `handler` / `inUsage`）。dispatch・ヘルプ・usage・補完はこのレジストリから生成される。
2. `cmdXxx(o *options, args []string) error` を責務ごとのファイルに書く（sync なら `sync.go`、status なら `status.go`）。新しい責務なら新ファイルを作る。パスは `o.root()` で得る（`os.UserHomeDir` を直接呼ばない）。
3. 位置引数の数が合わないときは `o.usageErr("err.usage.<cmd>")` を返す（文言は internal/i18n のカタログに追加する）。
4. `internal/i18n` の ja / en 両カタログに `usage.summary.<cmd>`（usage のコマンド一覧の説明）と `help.<cmd>`（`provsync <cmd> --help` の本文）を追加する（ja が正）。
5. README の「コマンドリファレンス」に足す。
6. テストを書く（上記の補助関数を使う）。

## 6. 新しいフラグを足す手順

1. `options` 構造体にフィールドを足す。
2. `registerFlags` に `fs.BoolVar(...)` / `fs.StringVar(...)` を足す。
3. 真偽値フラグは `reorder` が自動で検出する（追加作業なし）。値を取るフラグは `--name value` と `--name=value` の両方が使える。
4. `usage` の「共通フラグ:」または該当コマンドの説明に足す。

## 7. 詰まったときの止まり方

次のどれかに当たったら、実装を続けず、状況を報告してユーザーに確認する。

- PBI の記述とコードの実態が食い違う（例: 書かれた関数が存在しない）。
- 外部依存を足さないと実現できない。
- 既存のテストを書き換えないと通らない（期待値の変更が仕様変更になる場合）。
- 秘密の扱いに関わる仕様の判断が要る。
- 3 回直しても `make check` が通らない。

## 8. 完了チェックリスト（PBI ごとに全項目）

- [ ] BDD シナリオごとに自動テストがあり、パスする
- [ ] `make check` が通る（出力の末尾まで確認する）
- [ ] 新しい文言は ja / en 両カタログに追加（ja が正）、エラーは具体的（何が・どこで・次にどうするか）
- [ ] README と CHANGELOG（`[Unreleased]`）を更新した
- [ ] 秘密の値が出力・テストの期待値に残っていない（テスト用のダミー文字列を除く）
- [ ] 外部依存を追加していない（`git diff go.mod` が空）
- [ ] コミットは対象ファイルを個別に add した
