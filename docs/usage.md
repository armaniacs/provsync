# 使い方

日常の同期の流れと、各機能の詳細。

!!! note
    CLI のメッセージは日本語で出力される。

## クイックスタート

```console
# kilocode の provider を中央設定へ取り込む(まずはプレビュー)
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

# 中央設定を opencode へ反映する
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
中央設定: /home/you/.config/provsync/config.json (2 providers)
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

## 中央設定

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

`version` は中央設定の形式バージョン(将来のマイグレーション用)。現行は `1`。

## モデルエイリアス

中央設定の `aliases` に、共通モデル名 → ツール名 → モデル ID の対応を書くと、`push` のときに各ツールが要求する ID 形式へ自動変換する。

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

- 変換は push の描画前にだけ行う。`pull` は ID を書き換えない(往復で情報を失わない)。
- `aliases` にないモデル名は変換せず素通しする。
- 当該ツール向けの ID が未定義のエイリアスは警告して素通しする。`--strict` を付けるとエラーで終了し、何も書かない。
- 組み込みの対応表はない。対応表の正はユーザー定義の `aliases` で、pull しても消えない。

## 経路別フォールバック(routes)

中央設定の `routes` に、エイリアス名ごとに provider キーの優先順を書くと、`push` のときに「使える経路」(apiKeyEnv が設定済み、または apiKeyEnv 不要の provider)を先頭から選び、選ばれなかった経路の provider は描画対象から除く。

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

- 判定は環境変数の有無のみ。通信しない。値は読まない・出力しない。
- どの経路も使えないときは警告して、provider は変更せず残す。
- `apiKeyEnv` が空の provider は「環境変数不要」として常に使える経路になる。
- `routes` は pull しても消えない。ツール別の上書きは未対応(全ツール共通のみ)。

## API の疎通確認(check)

`provsync check` は中央設定の各 provider について、`baseURL` の `/models` へ認証付きの GET を送り到達可否を表示する。

- 通信するのは `check` だけ。他のコマンドは同期対象のファイル以外に一切アクセスしない。
- キーの値は環境変数から読むが、出力・ログには現れない。401/403 は `[認証失敗]`、タイムアウトや接続失敗は `[到達不可]` として分類表示する。
- `apiKeyEnv`・環境変数・`baseURL` が未設定の provider は通信せず `[スキップ]` と表示する。

## 環境の診断(doctor)

`provsync doctor` はパス解決・中央設定と各ツール設定の存在と構文・`apiKeyEnv` の環境変数の設定有無・ファイル権限を一覧で診断する。通信せず、ファイルも書かない。NG があるときは終了コード 1 で終わる。

```console
$ provsync doctor
[OK] パス解決
[OK] 中央設定: 2 providers
[警告] apiKeyEnv llm-01: 環境変数 LLM01_API_KEY が未設定です
[OK] ツール kilocode: /home/you/.config/kilo/kilo.jsonc
診断結果: OK 3 件 / 警告 1 件 / NG 0 件
```

## TUI ダッシュボード(任意)

`provsync-tui` はターミナル上で provider と同期先ツールの対応をチェックボックスで選び、確認画面の承認後に適用する。コアの `go.mod` に依存を持ち込まないため、`tui/` に独立したモジュールとして隔離されており、導入は任意。

```bash
cd tui
go build -o provsync-tui .
./provsync-tui
```

- TUI は `provsync` バイナリを子プロセスとして呼ぶ(`provsync status --json` で状態取得、`provsync push <tool> --provider <p> --write` で適用)。変更内容を自前で計算しない。
- 確認画面で承認するまで、ファイルは一切変わらない。
- 標準入力が端末でないときは「対話が必要です」でエラー終了する。
- `PROVSYNC_BIN` 環境変数で provsync バイナリのパスを指定できる(既定は PATH の `provsync`)。

## 秘密情報の扱い

- 中央設定が持つのは `apiKeyEnv`(環境変数名)のみ。実キーは仲介しない。
- `pull` 時、秘密情報らしいフィールド(`apiKey` / `api_key` / `token` / `secret` / `password` / `accessToken` / `access_token`。`options` 内も含む)は警告のうえ取り込まない。
- `push` 時、対象ツール設定に既に存在する秘密フィールドはそのまま保持される(削除も中央への持ち出しもしない)。
- opencode には `apiKeyEnv` を書き出さない(秘密は `auth.json` で管理されるため)。
- `diff` の出力では、秘密情報らしいキーの値を既定で `********` に伏せる。書き込まれるファイルの内容はマスクされない(表示のみ)。
- 手元で実値を確認したい場合は `--show-secrets` を付ける。実値が表示され、先頭に警告が出る。
- `status` / `list` はキー名の警告のみを表示し、値を出力しない。
- 脆弱性の報告方法は[セキュリティ](security.md)を参照。

## 再直列化に関する注意

ツール設定は全体が再直列化される(2 スペースインデント・キー辞書順)。元のコメントや書式は失われ、内容が同じでも `diff` に書式差分が混ざることがある。
