# provsync マルチツール provider 同期ツール 設計

日付: 2026-10-02
状態: 実装済み(0.2.0 としてリリース。本文は実装に合わせて as-built として維持する)

置き換え: `docs/superpowers/specs/2026-10-01-kilo-to-opencode-provider-port-design.md`(単発移植 CLI)を置き換える。旧 CLI のフラグは削除する。

## 目的

複数の LLM コーディングツール(kilocode / opencode など)がそれぞれ独自形式で持つ provider 設定を、
中央のカノニカル設定を唯一の正として相互変換・同期する Go 製 CLI を提供する。

日常的に安全に使えることを最優先し、次を満たす。

- 既定はプレビュー。`--write` を付けたときだけファイルを書き換える。
- 書き込み前に自動でバックアップし、`provsync undo` で元に戻せる。
- 「diff で見た内容」「実際に書かれる内容」「undo で戻す内容」が必ず一致する。

## 非目標(YAGNI)

- API キーなどの秘密情報の仲介・保管。provsync は秘密ストアを読み書きしない。
- 双方向の自動マージやファイル監視。
- ツール設定の provider 以外のセクションの意味的な同期(ただし書き込み時に保持はする)。
- 順序・コメント・整形の完全保持(全体再シリアライズを許容する)。
- v1 での 3 ツール目以降の実装(アダプタ IF で拡張可能にする)。

## 決定事項(ブレインストーミングの結論)

1. 中央設定を唯一の正(canonical)とする。ツール設定はそこから生成される派生物。
2. カノニカル表現はハイブリッド: 既知フィールドは正規化、ツール固有の未知フィールドはツール別に保持して往復無損失にする。
3. push は canonical に存在する provider(managed)のみ上書きし、ツール側にしか無い provider は保持する。
4. pull / push / sync は既定プレビュー。`--write` で適用し、バックアップを自動記録する。
5. 秘密は仲介しない。中央は `apiKeyEnv`(環境変数名)参照のみを持ち、実キーは各ツールのストアに委ねる。
6. v1 の対応ツールは kilocode と opencode の 2 つ。
7. 中央設定は JSON 固定(標準ライブラリのみ。コメントは扱わない)。
8. 旧フラグ駆動 CLI は完全置換。CHANGELOG に破壊的変更として記載する。
9. 安全機構として、変更計画(Plan)を単一情報源にし、タイムスタンプ付きバックアップと `undo` を提供する。
10. 差分表示は「意味差分 + 対象ファイルの統合 diff」の両方。

## アーキテクチャ

単一バイナリ。標準ライブラリのみを使う。

```
main.go                    サブコマンドのディスパッチのみ
internal/cli/              各コマンドのフラグ解析とハンドラ
internal/model/            カノニカル表現(Config / Provider)
internal/adapter/          Adapter IF + レジストリ + kilocode/opencode 実装
internal/store/            中央 config.json の load/save
internal/syncer/           provider 集合のマージ・絞り込み(純粋)
internal/jsonc/            JSONC(行コメント・末尾カンマ)の前処理
internal/plan/             変更計画(Plan)の生成。プレビュー/diff/書き込み/undo の単一情報源
internal/diff/             統合 diff のレンダリング
internal/backup/           タイムスタンプ付きバックアップ・マニフェスト・復元
internal/fsutil/           atomic write・ソート済み JSON 整形
```

### 責務と境界

- `model`: ツール非依存のデータ型のみ。I/O を持たない。
- `adapter`: ツール固有スキーマの解釈と描画のみ。コアは canonical しか知らない。
- `plan`: 読み込んだ状態から「どのファイルの Before/After」を決める純粋に近い層。
- `backup` / `fsutil`: 副作用(ファイル書き込み)を一手に引き受ける。
- `cli`: 入出力とフラグ。ビジネスロジックを持たない。

## データモデル

### カノニカル表現(`internal/model`)

```go
type Config struct {
    Version   int                 `json:"version"`
    Providers map[string]Provider `json:"providers"`
}

type Provider struct {
    Name      string                    `json:"name,omitempty"`
    NPM       string                    `json:"npm,omitempty"`
    BaseURL   string                    `json:"baseURL,omitempty"`
    APIKeyEnv string                    `json:"apiKeyEnv,omitempty"`
    Models    map[string]any            `json:"models,omitempty"`
    Extras    map[string]map[string]any `json:"x,omitempty"` // ツール名 -> 未知フィールド群
}
```

- 既知フィールドは `name` / `npm` / `baseURL` / `apiKeyEnv` / `models`。
- `models` は AI SDK 由来の共有形状として深く型化せず透過させる。空の `models` は中央設定への保存時に省略され、比較では nil と空オブジェクトを等価として扱う(偽差分を防ぐ)。
- 未知フィールドは `Extras[<tool>]` に保存し、同じツールへ push するときにそのまま戻す。
- `version` は将来のマイグレーション用。v1 は `1`。

### 中央設定ファイルの例

```json
{
  "version": 1,
  "providers": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "baseURL": "https://example.invalid/v1",
      "apiKeyEnv": "LLM01_API_KEY",
      "models": { "model-a": { "name": "model-a" } },
      "x": { "kilocode": { "reasoning": true } }
    }
  }
}
```

### Adapter インターフェース(`internal/adapter`)

```go
type Root struct {
    ConfigHome string // $XDG_CONFIG_HOME 相当
    StateHome  string // $XDG_STATE_HOME 相当
}

type Adapter interface {
    Name() string                                            // "kilocode" | "opencode"
    Path() string                                              // ツール設定ファイルの絶対パス(root は Get(name, root) で注入)
    Pull() (map[string]model.Provider, []string, error)      // ツール設定 -> canonical(2つ目は警告)
    Push(managed map[string]model.Provider) ([]byte, error)  // canonical を既存文書へマージして新ファイル内容を返す
    Project(managed map[string]model.Provider) map[string]model.Provider // managed をこのツールへの push が描画する形へ写す(意味差分の比較用)
}
```

- `Pull` / `Push` はアダプタが現在のツール設定ファイルを自ら読み込む。
- `Push` は provider 以外の設定と managed 外の provider を保持した「ファイル全文」を返す。
- `Root` は `--root` で差し替え可能とし、テストは一時ディレクトリで実ファイルを再現する。
- v1 実装:
  - `kilocode`: `$XDG_CONFIG_HOME/kilo/kilo.jsonc`(既定 `~/.config/kilo/kilo.jsonc`)、JSONC。
  - `opencode`: `$XDG_CONFIG_HOME/opencode/opencode.json`、JSON。
  - 別名 `kilo` は `kilocode` として受理する。

### フィールド対応(v1)

| canonical | kilocode (`kilo.jsonc`) | opencode (`opencode.json`) |
|---|---|---|
| `name` | `provider.<k>.name` | `provider.<k>.name` |
| `npm` | `provider.<k>.npm` | `provider.<k>.npm` |
| `baseURL` | `provider.<k>.options.baseURL` | `provider.<k>.options.baseURL` |
| `apiKeyEnv` | `provider.<k>.options.apiKeyEnv`(平文 `apiKey` は取り込まない) | 描画しない(秘密は `auth.json` が管理) |
| `models` | `provider.<k>.models` | `provider.<k>.models` |
| その他 | `Extras["kilocode"]` | `Extras["opencode"]` |

- 実際のフィールド形は実フィクスチャで検証済み。ツール間の差異はアダプタ内で吸収する。
- `apiKeyEnv` は情報として保持するが、opencode へは書き出さない(秘密を仲介しない方針の帰結)。
- 秘密情報らしいキー(`apiKey` / `token` 等)は pull 時に値を取り込まず警告する。
  push 時は、対象ツール設定ファイル内の既存の当該キーをそのまま保持する
  (中央経由では運ばず、同一ファイル内で書き換えによって失われないようにする)。
- 意味差分(push/diff のプレビュー、status の差分判定)は `Project` でツール可視の形へ写してから比較する。
  ツールが描画しないフィールド(apiKeyEnv や他ツールの extras)を差分として誤表示しないため。

## CLI 仕様

`provsync <command> [flags] [args]`。

### グローバルフラグ

| フラグ | 既定 | 説明 |
|---|---|---|
| `--root <dir>` | `$HOME` | すべてのパス解決の基準を差し替える(テスト・検証用) |
| `--write` | `false` | `true` のときだけファイルを更新する |
| `--no-backup` | `false` | バックアップを記録しない(非推奨) |
| `--provider <names>` | 未指定=全 managed | カンマ区切りで対象 provider を限定する(繰り返し可) |
| `--help` | | ヘルプ |

### コマンド

| コマンド | 書き込み | 説明 |
|---|---|---|
| `provsync list` | なし | 対応ツール名・設定パス・存在有無・provider 件数を表示 |
| `provsync status [tool...]` | なし | 各ツールと中央の状態、canonical との差分概要を表示 |
| `provsync pull <tool>` | `--write` 時 | ツール設定を canonical に取り込み、中央設定を更新 |
| `provsync push <tool>` | `--write` 時 | canonical の managed provider をツール設定へ反映 |
| `provsync sync --from <a> --to <b>` | `--write` 時 | a を canonical に取り込み、b へ反映(中央も更新) |
| `provsync diff <from> <to>` | なし | `<from>` を `<to>` に適用した場合の意味差分 + 統合 diff を表示 |
| `provsync undo [id]` | あり(復元) | 直前または指定操作を復元。`undo --list` で履歴表示 |

補足:

- `sync` の `--from` は省略可。省略時は中央設定の現状を `--to` へ反映する。
- `diff <from> <to>` は `sync --from <from> --to <to>` の Plan を表示するだけで書き込まない。中央設定の変更も含めて表示する。
- `undo` は復元操作自体を新しい操作として記録する(redo 可能)。
- `undo` は復元が目的のため、`--write` を要求せず直接適用する(グローバル `--write` は無視する)。やり直し用の操作 ID を表示する。
- `--no-backup` での書き込みは `Files` 無しのマーカー操作として履歴に記録される。後続の `undo` は、直近の書き込みがバックアップなしで行われた旨を警告してから、それより前の操作を復元する。
- `status` / `list` / `diff` は読み取り専用で、drift があっても終了コード 0(v1 では検査用の非 0 終了は提供しない)。
- 変更が無い場合、プレビューは `変更はありません` を表示し、`--write` は書き込みをスキップする(冪等)。

## 変更計画(Plan)と安全機構

### Plan(単一情報源)

```go
// internal/plan
type Plan struct {
    Changes []FileChange
}

type FileChange struct {
    Tool     string // "central" | "kilocode" | "opencode"
    Path     string
    Before   []byte
    After    []byte
    Semantic []ProviderChange
}

type ProviderChange struct {
    Key    string   // provider 名
    Op     string   // added | updated | unchanged
    Fields []string // 変更された canonical フィールド(extras を含む)
}
```

- `pull` / `push` / `sync` / `diff` はすべて Plan を生成する。
- `diff` とプレビューは Plan を描画するだけ。`--write` は同じ Plan を適用する。
- Plan は「Before と After のバイト列」を持つため、表示・適用・バックアップが同一内容に固定される。

### バックアップ

```
$XDG_STATE_HOME/provsync/
  index.json                        操作マニフェスト(新しい順、最新 20 件を保持)
  backups/<op-id>/<n>-<basename>   ファイルコピー(パーミッションも保存)
```

```go
type Operation struct {
    ID        string     // 例: 20261002T085512-<rand>
    StartedAt time.Time
    Command   string     // "push opencode" 等
    Files     []FileRef  // {Path, BackupPath, SHA256, Mode}
    Undone    bool
    NoBackup  bool       // --no-backup での書き込みを示すマーカー(Files は空で、undo の対象外)
}
```

- Plan を適用する直前に、影響する全ファイルを1操作としてバックアップする。
- バックアップと `index.json` の更新も atomic に行う。
- 最新 20 操作を超えた古いものは削除する。

### undo

- `undo`: 直近の操作(未 undo の最新)を復元する。
- `undo <id>`: 指定操作を復元する。
- `undo --list`: 操作履歴(ID / 日時 / コマンド / 対象ファイル / undone)を表示する。
- 復元は各ファイルをバックアップから atomic に書き戻し、権限も復元する。
- 復元前の現状を新しい操作として記録し、元操作を `Undone=true` にする。これにより undo 自体を undo できる。
- redo 用の記録は復元より先にマニフェストへ反映する。復元の途中で失敗しても redo は常に `index.json` から到達可能。
- `--no-backup` のマーカー操作は bare `undo` の対象外(スキップされる)。ただし最新の操作がマーカーのときは、それより前の操作に戻る旨を警告する。

## データフロー

### push `<tool>`

1. 中央設定を読み込む(無ければエラー。「先に pull するか sync を使ってください」と案内)。
2. `--provider` 指定があれば canonical に存在しない名前はエラー。
3. アダプタが現在のツール設定を読み、canonical を managed としてマージした全文(`After`)を生成。
4. Plan(対象: ツール設定)を生成。
5. プレビュー表示、または `--write` でバックアップ + atomic write。

### pull `<tool>`

1. アダプタがツール設定を読み、canonical へ変換。
2. 中央設定の該当 provider をマージ(managed = 取得した provider、`--provider` で限定可)。
   ツール側に存在しない provider は中央から削除しない(安全側。削除は将来の明示コマンドで扱う)。
   マージは該当ツールの名前空間(`Extras[tool]`)のみを上書きし、他ツールの名前空間と `version` は保持する。
3. Plan(対象: 中央設定)を生成。
4. プレビュー表示、または `--write`。

### sync `--from a --to b`

1. `a` から canonical を取り込み、中央設定を更新する Plan を作る。
2. 更新後の canonical を `b` へマージした Plan を加える。
3. 2 つの FileChange(中央 + `b`)をまとめてプレビュー / 適用。

### diff `<from> <to>`

- sync と同じ Plan を生成し、書き込まずに「意味差分 → 統合 diff」の順で表示する。

## 差分表示

1. 意味差分: provider 単位に `added` / `updated` / `unchanged` と変更フィールドを表示。
2. 統合 diff: 対象ファイルごとに `Before` / `After` の unified diff を git diff 風(`---`/`+++`、`@@`、`+`/`-`)で表示。

注意(許容事項): 書き出しは JSON 全体の再シリアライズ(2 スペースインデント、トップレベルキーはアルファベット順)である。したがってツール側の元の整形・コメントは失われ、diff に整形差分が含まれ得る。これは許容する。

## エラー処理

| 状況 | 挙動 |
|---|---|
| 未知のツール名 | 有効なツール名を列挙して非 0 終了 |
| pull 時のツール設定が無い | 非 0 終了(読めない旨を表示) |
| push 時のツール設定が無い | 非 0 終了(新規作成はしない) |
| push 時に中央設定が無い | 非 0 終了(pull / sync を案内) |
| JSON / JSONC が不正 | パス付きで非 0 終了 |
| `--provider` の名前が canonical に無い | 非 0 終了 |
| pull で平文 `apiKey` を検出 | 警告を出して取り込まない(秘密は仲介しない)。続行 |
| 書き込み失敗 | バックアップ記録済み。一時ファイル + rename のため元ファイルは無傷。非 0 終了 |
| 変更なし | プレビューは `変更はありません`。`--write` は書き込まず正常終了 |
| `undo` の履歴が無い / ID が無い | 非 0 終了 |

## パス解決

- 中央設定: `$XDG_CONFIG_HOME/provsync/config.json`(既定 `~/.config/provsync/config.json`)。
- 状態: `$XDG_STATE_HOME/provsync/`(既定 `~/.local/state/provsync/`)。
- ツール設定: 各アダプタが `Root` から解決。
- `--root <dir>` 指定時は `ConfigHome` / `StateHome` をその配下に置き換える(テスト用)。

## テスト方針(必須)

すべてのパッケージにテストを置く。TDD で失敗テストから書く。`make check`(fmt-check → vet → test)を必ず通す。

- `model`: extras の marshal/unmarshal 往復。既知フィールドの省略。
- `adapter/kilocode`: JSONC 解析、既知 + 未知フィールドの変換、平文 apiKey の警告・非取込、描画時に他キーと managed 外 provider を保持。
- `adapter/opencode`: JSON 解析・描画、provider 以外の設定保持、apiKeyEnv を書き出さないこと。
- `syncer`: managed マージで managed 外を保持、意味差分の計算(added/updated/unchanged)。
- `store`: load/save、欠落時の挙動、atomic 書き込み。
- `backup`: 操作の記録、一覧、最新 20 件の保持と剪定、`undo` による復元がバイト単位で一致、undo が新操作を記録すること。
- `diff`: 既知の Before/After に対する unified diff のゴールデン比較。
- `plan`: Plan 生成と意味差分。
- `cli`(`run(args, out)` 統合テスト、`--root` に `t.TempDir()` を使用):
  - `list` / `status` / `pull` / `push` / `sync` / `diff` / `undo` の各コマンド。
  - プレビューがファイルを変更しないこと。`--write` が期待どおり書き込み、バックアップを作ること。
  - extras の往復で情報が失われないこと。
  - 冪等性(同内容の再 push で `変更はありません`)。
  - エラー系(未知ツール、欠落ファイル、不正 JSON、未知 provider)。
  - `undo` で `--write` 前の状態へバイト単位で復元されること。

## ドキュメント更新

実装時に以下を更新する。

- `README.md`: 新サブコマンド、中央設定、バックアップ / undo の説明。
- `AGENTS.md`: 新パッケージ構成、テスト・チェック手順、gotcha(秘密を仲介しない、`--write` 既定で書き込まない等)。
- `CHANGELOG.md`: 破壊的変更(旧フラグ削除)と新機能を追記。

## 実装順序(概要)

詳細は別途 writing-plans で計画化する。

1. `model` / `fsutil`: 型と既存 I/O の抽出。
2. `store`: 中央設定の load/save。
3. `adapter`: IF + kilocode / opencode 実装。
4. `syncer` / `plan`: 意味差分と Plan 生成。
5. `backup`: マニフェスト、記録、復元、剪定。
6. `diff`: unified diff レンダリング。
7. `cli` + `main.go`: サブコマンド実装、旧 CLI 削除。
8. ドキュメント更新と `make check`。