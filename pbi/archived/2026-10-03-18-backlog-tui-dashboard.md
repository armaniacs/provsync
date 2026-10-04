# PBI: TUI ダッシュボードと API 疎通確認

## ユーザーストーリー
provsync の利用者として、ターミナル上で provider と同期先ツールの対応を一覧し、チェックボックスで選んで適用したい、なぜならサブコマンドとフラグを覚えなくても、どのツールにどの provider が反映されるかを見ながら安全に操作したいから

## 優先度
- 順位: 18 / 19
- RICEスコア: 0.83（Reach=5 / Impact=1 / Confidence=50% / Effort=3）
- 根拠: 工数が大きく、既存の `--provider` 指定と `status` / `diff` で同じ操作は既にできるため増分価値は小さい。方針上の論点（依存追加・ネットワーク通信）が未決のため、論点を決めてから着手する。パス解決（11）の確定後に作る

## 設計方針（なぜなぜ分析の結果）
- 疑問: 「完成度とバズり度」は何の価値か
  - 表面は見栄えだが、本質は「同期対象の選択ミスを実行前に視覚で防ぐ」こと。ゆえに TUI は新しいロジックを持たず、既存の Plan を表示・選択する薄い層にする
- 疑問: AGENTS.md の「標準ライブラリのみ」と矛盾しないか
  - Bubbletea は外部依存。標準ライブラリだけでは raw モードと端末制御が重い
  - 解: コアは標準ライブラリのみを維持する。TUI は別モジュール（例: `cmd/provsync-tui` に独立した go.mod）または別バイナリに隔離し、導入は利用者の任意にする。方針の変更は決定記録として残す
- 疑問: 疎通確認（Ping）は秘密の方針と両立するか
  - 現状、秘密は中継せず `apiKeyEnv`（環境変数名）だけを持つ。疎通確認は環境変数の値をプロセス内で読み、送信先へ認証付きで要求する。値は表示・保存しない
  - 解: 疎通確認は TUI と独立した CLI コマンド（例: `provsync check`）として先に作り、明示実行のみ（同期系コマンドは通信しない）にする。TUI はそれを呼ぶだけにする
- Reach=5 と置いた根拠: 想定利用者のうち TUI を好む層のみ

## BDD受け入れシナリオ
Scenario: provider と同期先をチェックボックスで選んで適用する
  Given 中央設定に複数の provider があり、kilocode と opencode の設定がある
  When  TUI で provider と同期先を選び、確認画面で承認する
  Then  選んだ組み合わせだけが Plan どおり書き込まれ、バックアップが記録される

Scenario: 確認前には何も書かれない
  Given TUI で選択を変更した
  When  確認画面で取り消す
  Then  ファイルは変更されない

Scenario: 疎通確認の結果を表示する
  Given provider の `apiKeyEnv` に対応する環境変数が設定されている
  When  `provsync check` を実行する
  Then  各 provider の到達可否と応答時間が表示され、キーの値は表示されない

Scenario: 環境変数が未設定の provider
  Given `apiKeyEnv` の環境変数が未設定である
  When  `provsync check` を実行する
  Then  その provider は「環境変数未設定」と表示され、通信は行われない

Scenario: 非対話端末では TUI を起動しない
  Given 標準入力が端末でない
  When  TUI を起動する
  Then  対話が必要である旨のエラーで終了する

## 受け入れ基準
- [x] TUI は Plan を再利用し、変更内容を独自計算しない
- [x] `check` はタイムアウトを持ち、同期系コマンドは通信しない
- [x] 秘密の値をログ・出力・バックアップに含めない
- [x] コアの `go.mod` に外部依存を追加しない
- [x] 依存の隔離方式を決定記録として残す

## テスト戦略
- E2E: 擬似端末での操作、`check` を `httptest` サーバに対して実行
- 統合: 選択結果から Plan が構築され、適用結果が CLI 経由と一致すること
- 単体: 選択状態のモデル、タイムアウトとエラー分類

## 見積もり
8 ポイント（要チームでの見積もり。`check` 単体は 2 ポイント）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 最初に確認（実装ゲート）
この PBI は未決事項がある。次を満たすまで TUI の実装に着手しない。満たされていなければ、下の「Step A: `check` コマンド」だけ実装して止まり、ユーザーに TUI の依存方針を確認する。

- ゲート 1: ユーザーが「TUI を別モジュールまたは別バイナリに隔離し、コアの `go.mod` に依存を足さない」方針を承認している。
- ゲート 2: 11（Windows 非対応と XDG テスト）が完了している。

外部依存（Bubbletea など）を、承認なしに `go.mod` へ足してはいけない。

### Step A: `provsync check`（標準ライブラリのみ・単独で実装可）
先に読む: `cmdStatus`、`store.Load`、`model.Provider`（`BaseURL` / `APIKeyEnv`）、`net/http/httptest`。

1. `Run` に `case "check"` と `usage` の 1 行。`check` は中央設定の各 provider（`--provider` で絞れる）について順に確認する。
2. 各 provider の処理:
   - `APIKeyEnv == ""` → `[スキップ] <key>: apiKeyEnv が未設定`（通信しない）。
   - `os.LookupEnv(APIKeyEnv)` が無い → `[スキップ] <key>: 環境変数 <名前> が未設定`（通信しない。値は出さない）。
   - `BaseURL == ""` → `[スキップ] <key>: baseURL が未設定`。
   - それ以外: `context.WithTimeout(ctx, 5*time.Second)` 付きで `GET <BaseURL>/models` を送る。ヘッダは `Authorization: Bearer <環境変数の値>`。結果は ステータス 2xx → `[OK] <key> (<ms>ms)`、401/403 → `[認証失敗] <key>`、その他 → `[応答異常] <key>: HTTP <code>`、タイムアウト・接続失敗 → `[到達不可] <key>: <エラー概要>`。
3. 秘密の値をログ・出力・エラー文字列に含めない（`http` のエラーに URL が含まれるが、ヘッダは含まれない。それでも出力するエラーは `err.Error()` ではなく分類済みの短い文言にする）。
4. 通信するのは `check` コマンドだけ。他のコマンドから呼ばない。
5. テスト: `httptest.NewServer` を `BaseURL` にして 200 → OK、401 → 認証失敗、遅延 → タイムアウト（短い timeout を注入できるよう `options` か変数で上書き可能にする）、環境変数未設定 → サーバーにリクエストが来ない（`atomic` カウンタで検証）、出力にトークンの実値が含まれない。

### Step B: TUI（ゲートを通過してから）
- 別ディレクトリ `tui/` に独立した `go.mod`（`module github.com/armaniacs/provsync/tui`）を置き、コア側の `go.mod` は無変更にする。TUI はコアの `internal` を import できないため、`provsync` バイナリを子プロセスとして呼ぶ方式（`provsync status --json`、`provsync push <tool> --provider ... [--write]`）にする。これには 12（`--json`）が必要。
- TUI は選択を集めて、確認画面でプレビューを見せ、承認されたときだけ `--write` 付きで子プロセスを実行する。変更内容を自前で計算しない。
- 標準入力が端末でないときは「対話が必要です」でエラー終了。

### 注意
- Step A だけで PBI の価値（疎通確認）の半分は満たせる。Step B の前に必ず報告する。
