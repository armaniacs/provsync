# 使い方

日常の同期の流れと、各機能の詳細。最初に通読するのは[クイックスタート](#クイックスタート)から[差分の確認と取り消し](#差分の確認と取り消し)までで、セントラル設定以降の節は必要になったときに引けば足りる。

!!! note
    CLI のメッセージは英語が既定。`PROVSYNC_LANG`(または `LC_ALL` / `LC_MESSAGES` / `LANG`)が `ja` で始まるロケールなら日本語で出力される。未対応のロケールは英語にフォールバックする。

## クイックスタート

```console
# kilocode の provider をセントラル設定へ取り込む(まずはプレビュー)
$ provsync pull kilocode
central: /home/you/.config/provsync/config.json
  llm-01: 追加
  llm-02: 追加
(プレビューのみ; 適用するには --write)

# --write で適用。書き込みの直前にバックアップが記録される
$ provsync pull kilocode --write
central: /home/you/.config/provsync/config.json
  llm-01: 追加
  llm-02: 追加
バックアップ: 20261002T093012-3fa1
書き込み: /home/you/.config/provsync/config.json

# セントラル設定を opencode へ反映する
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  llm-01: 追加
  llm-02: 追加
バックアップ: 20261002T093045-8c2d
書き込み: /home/you/.config/opencode/opencode.json

# 同じ状態への再書き込みは冪等
$ provsync push opencode --write
opencode: /home/you/.config/opencode/opencode.json
  変更なし
変更はありません
```

## 同期状態の確認

```console
$ provsync status
セントラル設定: /home/you/.config/provsync/config.json (2 providers)
kilocode  /home/you/.config/kilo/kilo.jsonc (2 providers)
  差分なし
opencode  /home/you/.config/opencode/opencode.json (3 providers)
  警告: 秘密情報らしいフィールド "options.apiKey" を検出しました。秘密は仲介しないため取り込みません
  groq: 中央に無い
```

ツール側にだけ存在する provider(上の `groq`)は「中央に無い」と表示される。`push` が対象ツール設定内の管理対象外エントリを削除することはない。

## 差分の確認と取り消し

```console
$ provsync diff kilocode opencode
opencode: /home/you/.config/opencode/opencode.json
  llm-01: 追加
  llm-02: 追加
--- a/home/you/.config/opencode/opencode.json
+++ b/home/you/.config/opencode/opencode.json
@@ -1,15 +1,44 @@
   {
 -  "theme": "dark",
   "provider": {
     ...

$ provsync undo
復元しました: 20261002T093045-8c2d (push opencode)
やり直し: provsync undo 20261002T100001-51b7
  復元: /home/you/.config/opencode/opencode.json
```

`undo` は `--write` を要求せず直接適用する。

## セントラル設定

既知フィールド(`name` / `npm` / `baseURL` / `apiKeyEnv` / `models`)は正規化して保持し、ツール固有の未知フィールドは `x.<tool>` に名前空間化して保存する。同じツールへ `push` するときに復元されるため、pull → push の往復で失われない。

```json
{
  "providers": {
    "llm-01": {
      "apiKeyEnv": "LLM01_API_KEY",
      "baseURL": "https://llm-01.example.com/v1",
      "models": {
        "model-a": { "name": "Model A" },
        "model-b": { "name": "Model B" }
      },
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "x": {
        "kilocode": { "reasoning": true }
      }
    }
  },
  "version": 1
}
```

`version` はセントラル設定の形式バージョン(将来のマイグレーション用)。現行は `1`。

## モデルエイリアス

セントラル設定の `aliases` に、共通モデル名 → ツール名 → モデル ID の対応を書くと、`push` のときに各ツールが要求する ID 形式へ自動変換する。

```json
{
  "aliases": {
    "sonnet": {
      "kilocode": "claude-sonnet-4-5",
      "opencode": "anthropic/claude-sonnet-4-5"
    }
  },
  "providers": {
    "llm-01": {
      "models": { "sonnet": { "name": "Sonnet" } }
    }
  }
}
```

モデル名の変換は push の描画前にだけ行われる。pull は ID を書き換えないため、push してから pull で戻しても情報は失われない。

`aliases` に書いていないモデル名は変換されず、通常のモデル ID としてそのまま書き出される。エイリアスは存在するのに当該ツール向けの ID が定義されていないときは、警告を出したうえでキーを素通しする。警告ではなくエラーで止めたいときは `--strict` を付ける。この場合 push はエラーで終了し、ファイルには何も書かれない。

組み込みの対応表は存在しない。対応表の正はユーザー定義の `aliases` だ。

## 経路別フォールバック(routes)

セントラル設定の `routes` に、エイリアス名ごとに provider キーの優先順を書くと、`push` のときに「使える経路」(apiKeyEnv が設定済み、または apiKeyEnv 不要の provider)を先頭から選ぶ。選ばれなかった経路の provider は描画対象から除かれる。

```json
{
  "routes": {
    "sonnet": ["anthropic", "openrouter"]
  },
  "providers": {
    "anthropic": { "apiKeyEnv": "ANTHROPIC_API_KEY", "models": { "sonnet": {} } },
    "openrouter": { "apiKeyEnv": "OPENROUTER_API_KEY", "models": { "sonnet": {} } }
  }
}
```

経路の判定は環境変数の有無だけで行われる。通信は発生せず、値を読むことも出力することもない。`apiKeyEnv` が空の provider は環境変数不要として、常に使える経路になる。

書ける経路が 1 つもないときは警告を出して、provider を変更せずに残す。どの経路も未設定のまま push しても設定が消えないのは、このためだ。`routes` 自体は pull で消えない（セントラル設定のトップレベルとして保持される）。

ツール別の上書きには対応していないため、経路の優先順は全ツール共通だ。

## API の疎通確認(check)

`provsync check` はセントラル設定の各 provider について、`baseURL` の `/models` へ認証付きの GET を送り到達可否を表示する。

通信するのは provsync の中で check だけだ。他のコマンドは同期対象のファイルと状態ディレクトリ以外にアクセスしない。キーの値は環境変数から読むが、出力にもログにも現れない。

結果は分類して表示される。401 / 403 は `[認証失敗]`、タイムアウトや接続の失敗は `[到達不可]`、そして `apiKeyEnv`・環境変数・`baseURL` のいずれかが未設定の provider は通信せず `[スキップ]` になる。

## 環境の診断(doctor)

`provsync doctor` はパス解決・セントラル設定と各ツール設定の存在と構文・`apiKeyEnv` の環境変数の設定有無・ファイル権限を一覧で診断する。通信せず、ファイルも書かない。NG があるときは終了コード 1 で終わる。

```console
$ provsync doctor
[OK] パス解決
[OK] セントラル設定
[警告] apiKeyEnv llm-01: 環境変数 LLM01_API_KEY が未設定です
[OK] ツール kilocode
[OK] ツール opencode
診断結果: OK 4 件 / 警告 1 件 / NG 0 件
```

## TUI ダッシュボード(任意)

`provsync-tui` はターミナル上で provider と同期先ツールの対応をチェックボックスで選び、確認画面の承認後に適用する。コアの `go.mod` に依存を持ち込まないため、`tui/` に独立したモジュールとして隔離されており、導入は任意。

```bash
make install    # provsync と provsync-tui をインストール
provsync-tui
```

手動でビルドする場合は `tui/` で `go build -o provsync-tui .` を実行する。

TUI は provsync バイナリを子プロセスとして呼ぶ構成で、状態の取得は `provsync status --json`、適用は `provsync push <tool> --provider <p> --write` で行う。変更内容の計算を TUI 側で持たないため、画面に見えたものと実際に書かれるものの整合は CLI 側の Plan に一任される。

確認画面には実行コマンドごとに `--write` なし push のプレビュー(意味差分)が表示されるため、何が変わるかを見てから承認できる。確認画面で承認するまで、ファイルは一切変わらない。適用後の完了画面では `u` で直前の操作を取り消せる(`undo` 相当の確認を挟んで実行する)。

差分が無いときは一覧の代わりにコマンドメニューが開く。一覧画面でも `m` で開ける。メニューは全サブコマンド(`status` / `list` / `pull` / `push` / `sync` / `diff` / `undo` / `doctor` / `check` / `init` / `version` / `completion`)を並べ、選んで引数を入れると実行する。書き込みを伴う操作は確認を挟み、`pull` / `push` / `sync` / `init` はプレビュー表示のうえで承認後に `--write` 付きで実行する。プレビュー自体が失敗したときは適用を受け付けず、戻る操作だけが有効になる。

全画面の下部には使えるキーのフッターが常時表示され、`?` でショートカット一覧のヘルプを開ける。一覧とメニューでは `g` / `G` で先頭・末尾へ飛べる。

標準入力が端末でない場合(CI 実行など)は「interaction required」で終了する。エラー文言の言語は PROVSYNC_LANG / LANG に従う(既定は英語、`ja` で始まるロケールでは日本語)。provsync バイナリの場所は `PROVSYNC_BIN` 環境変数で指定でき、既定では PATH にある `provsync` を使う。

## 秘密情報の扱い

セントラル設定が持つのは `apiKeyEnv`(環境変数名)だけで、実キーは仲介しない。pull のときに秘密情報らしいフィールド(`apiKey` / `api_key` / `token` / `secret` / `password` / `accessToken` / `access_token`。`options` 内も含む)があると、警告を出したうえでそのフィールドを落とす。

push では逆に、対象ツール設定にすでに存在する秘密フィールドをそのまま保持する。削除も、セントラル設定への持ち出しもしない。opencode には `apiKeyEnv` を書き出さない。秘密は opencode 側で `auth.json` が管理しているためだ。

diff の出力では、秘密情報らしいキーの値を既定で `********` に伏せる。あくまで表示上のマスクで、書き込まれるファイルの内容には影響しない。手元で実値を確認したいときは `--show-secrets` を付ける。実値が表示され、先頭に警告が出る。status と list はキー名の警告だけを出し、値を出力しない。

脆弱性の報告方法は[セキュリティ](security.md)を参照。

## 再直列化に関する注意

ツール設定は全体が再直列化される(2 スペースインデント・キー辞書順)。元のコメントや書式は失われ、内容が同じでも `diff` に書式差分が混ざることがある。
