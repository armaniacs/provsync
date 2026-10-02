# PBI: Windows 非対応の明示と XDG 解決の回帰テスト

## ユーザーストーリー
Windows で provsync を起動した利用者として、未対応であることを最初に明確なエラーで知りたい、なぜなら設定パスの前提（`~/.config` 系）が合わずに、意図しない場所へファイルを作る事故を避けたいから

## 優先度
- 順位: 11 / 19
- RICEスコア: 4.8（Reach=3 / Impact=0.5 / Confidence=80% / Effort=0.25）
- 根拠: コード調査の結果、`XDG_CONFIG_HOME` / `XDG_STATE_HOME` の解決は `adapter.NewRoot` に実装済みと分かった。残るのは Windows 非対応の明示（決定済み）と、XDG 解決の回帰テストの追加のみで工数が小さい。

## BDD受け入れシナリオ
Scenario: XDG_CONFIG_HOME を尊重する（回帰テスト）
  Given `XDG_CONFIG_HOME=/custom` が設定されている
  When  Root を解決する
  Then  中央設定のパスが `/custom/provsync/config.json` になる

Scenario: 未設定なら従来どおり
  Given `XDG_CONFIG_HOME` が未設定である
  When  Root を解決する
  Then  `~/.config/provsync/config.json` になる

Scenario: Windows は未対応と分かる
  Given Windows 上で実行する
  When  provsync を起動する
  Then  Windows は未対応である旨が stderr に出て、終了コード 1 で終了する

## 受け入れ基準
- [x] `adapter.NewRoot` の XDG 解決をテーブルテストで固定する（実装の変更は不要）
- [x] 起動時の OS 判定を `cli.Supported(goos string) error` として切り出し、`main.go` から呼ぶ
- [x] README の対応環境に「macOS / Linux。Windows は非対応」と明記する
- [x] 06（リリース）の対象 OS は darwin / linux のみ

## テスト戦略
- E2E: 対象外
- 統合: 対象外
- 単体: `NewRoot` の環境変数ケース、`Supported("windows")` がエラー・`Supported("linux")` が nil

## 見積もり
1 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`main.go`、`internal/adapter/adapter.go` の `NewRoot` / `ResolveRoot`、`internal/adapter/adapter_test.go`。

### 事実の整理（調査済み）
`XDG_CONFIG_HOME` と `XDG_STATE_HOME` の解決は `NewRoot` に実装済み。新しい実装は要らない。足すのは回帰テストと Windows 判定だけ。

### 手順
1. `internal/adapter/adapter_test.go` に `TestNewRoot` を追加（テーブルテスト、`t.Setenv` を使う。`t.Setenv("XDG_CONFIG_HOME", "")` で未設定扱いにできる）:
   - 両方未設定 → `ConfigHome == filepath.Join(home, ".config")`、`StateHome == filepath.Join(home, ".local", "state")`
   - `XDG_CONFIG_HOME=/custom` → `CentralConfigPath() == "/custom/provsync/config.json"`
   - `XDG_STATE_HOME=/s` → `StateDir() == "/s/provsync"`
2. `internal/cli/platform.go` を作る。

```go
package cli

import "fmt"

// Supported は実行 OS が対応環境かを返す。
func Supported(goos string) error {
	if goos == "windows" {
		return fmt.Errorf("Windows は未対応です(対応: macOS / Linux)")
	}
	return nil
}
```
3. `main.go` の `cli.Run` の前に追加（`runtime` を import）:

```go
if err := cli.Supported(runtime.GOOS); err != nil {
	fmt.Fprintln(os.Stderr, "provsync:", err)
	os.Exit(1)
}
```
4. `internal/cli/platform_test.go`: `Supported("windows")` が error、`Supported("linux")` と `Supported("darwin")` が nil。
5. README に対応環境（macOS / Linux、Windows は非対応）を日本語・English 両方に追記。CHANGELOG `[Unreleased]`。

### 注意
- `main.go` は「起動のみ」の方針。判定ロジックは `cli.Supported` に置く。
- XDG のロジックを書き換えない（テストだけ足す）。
