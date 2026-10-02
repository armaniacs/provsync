# PBI: 機械可読な出力（--json）と drift 検出の終了コード

## ユーザーストーリー
スクリプトや dotfiles の自動化で provsync を使う開発者として、`status` や `list` の結果を JSON で受け取り、差分があるかを終了コードで判定したい、なぜなら人向けの日本語出力を解析するのは壊れやすいから

## 優先度
- 順位: 12 / 19
- RICEスコア: 3.2（Reach=4 / Impact=1 / Confidence=80% / Effort=1）
- 根拠: 自動化用途に必要だが、対象は一部の利用者。終了コードの体系は 07（ヘルプ・終了コード）で統一するため、その後に行う

## BDD受け入れシナリオ
Scenario: status を JSON で出力する
  Given 中央設定と kilocode の間に差分がある
  When  `provsync status --json` を実行する
  Then  標準出力は妥当な JSON で、ツールごとの状態と provider ごとの差分種別を含む

Scenario: 差分の有無を終了コードで判定する
  Given 中央設定と opencode が同期済みである
  When  `provsync status --exit-code` を実行する
  Then  終了コードは 0 で、差分があれば 3 になる

Scenario: JSON に秘密を含めない
  Given 秘密を含むファイルがある
  When  `provsync diff kilocode opencode --json` を実行する
  Then  秘密の値はマスクされる

## 受け入れ基準
- [x] `list` / `status` / `diff` に `--json` を追加する。スキーマは README に文書化し、`schemaVersion` を含める
- [x] `--json` 時は警告を標準エラーへ出し、標準出力は JSON のみとする
- [x] 終了コードの割り当て（0 成功 / 1 実行時エラー / 2 使い方の誤り / 3 差分あり）を 07 の体系に合わせる
- [x] 秘密のマスクは 04 の仕組みを共有する

## テスト戦略
- E2E: 実バイナリの出力を `jq` 相当でパースし終了コードを確認
- 統合: `cli.Run` で 3 シナリオ
- 単体: JSON スキーマのゴールデン比較

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`cmdStatus` / `driftLines` / `cmdList` / `cmdDiff`、`internal/plan/plan.go`（`ProviderChange`）、07 で作る `errors.go`（`UsageError`）と `RunWith`。04 の `internal/secret`。

### 設計の確認
重複を避けるため、`status` を「集計して構造体にする」部分と「表示する」部分に分ける。テキスト出力は今までと同一にし、`--json` のときだけ構造体を JSON で出す。

### 手順
1. `options` に `json bool`、`exitCode bool` を足し、`registerFlags` に `--json` / `--exit-code` を足す。
2. `cmdStatus` を次の構造に直す（既存の挙動・文言は維持）:

```go
type statusReport struct {
	SchemaVersion int          `json:"schemaVersion"`
	Central       centralInfo  `json:"central"`
	Tools         []toolStatus `json:"tools"`
}
type centralInfo struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Providers int    `json:"providers"`
}
type toolStatus struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Exists    bool     `json:"exists"`
	Providers int      `json:"providers"`
	Warnings  []string `json:"warnings"`
	Drift     []string `json:"drift"` // driftLines の戻り値の「差分なし」を除いたもの
}
```
   `buildStatusReport(o, root, tools) (statusReport, error)` を作り、`cmdStatus` はそれを呼んで、`o.json` なら `json.MarshalIndent` で `o.out` へ、そうでなければ従来のテキストを出す。`Drift` が空 = 差分なし。
3. 差分あり判定: `Drift` が 1 件以上あるツールが 1 つでもあれば「差分あり」。`o.exitCode` かつ差分ありなら終了コード 3 にする。

```go
// ExitError は特定の終了コードで終了させるためのエラー。メッセージは出さない。
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit %d", e.Code) }
```
   `cmdStatus` が `&ExitError{Code: 3}` を返し、`main.go` は `errors.As` で判別して、メッセージを出さずにその Code で終了する。
4. `list` も同様に `--json`（`central` と `tools` の配列）。
5. `diff --json`: 各 `FileChange` について `{tool, path, semantic: [...], diff: "<masked unified diff>"}`。diff の文字列は 04 の `secret.MaskLines` を通す（`--show-secrets` なしは必ずマスク）。
6. `--json` のとき、警告などの人向けメッセージは `o.errOut`（stderr）へ出し、`o.out` は JSON だけにする。
7. `docs`: README に JSON のスキーマ（上の構造体）と終了コード表（0 成功 / 1 実行時エラー / 2 使い方 / 3 差分あり）を書く。
8. テスト: `--json` の出力が `json.Unmarshal` できる、`schemaVersion == 1`、差分ありで `--exit-code` が `*ExitError{3}`、同期済みで nil、`diff --json` の出力に秘密の実値が含まれない。

### 注意
- テキスト出力の文言を変えない（既存テストが依存している）。
- JSON のキー名・構造は後から変えにくい。上の構造体どおりに作り、勝手に増やさない。
