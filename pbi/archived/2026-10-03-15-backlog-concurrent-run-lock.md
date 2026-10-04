# PBI: 多重起動時の排他制御

## ユーザーストーリー
provsync の利用者として、複数のプロセスが同時に書き込んでも履歴や設定が壊れないでほしい、なぜなら dotfiles の自動適用やシェルのフックから並行して呼ばれうるから

## 優先度
- 順位: 15 / 19
- RICEスコア: 2.4（Reach=3 / Impact=1 / Confidence=80% / Effort=1）
- 根拠: 個々のファイル書き込みは atomic だが、バックアップの索引（読み→更新→保存）は複数プロセスで競合しうる。発生頻度は低いが、起きると undo の履歴が壊れる。実運用で並行実行が増える（12 の自動化）前に備える

## BDD受け入れシナリオ
Scenario: 同時実行は後発が待つか明確に失敗する
  Given 1 つ目のプロセスが `--write` の途中である
  When  2 つ目の `provsync push opencode --write` を起動する
  Then  待機後に実行されるか、タイムアウトして「別の provsync が実行中」と表示して終了する

Scenario: プレビューはロックを取らない
  Given 別のプロセスが書き込み中である
  When  `provsync diff kilocode opencode` を実行する
  Then  待たずに結果が表示される

Scenario: 異常終了で残ったロックは自動回復する
  Given 前回のプロセスが強制終了されロックファイルが残っている
  When  `provsync push opencode --write` を実行する
  Then  古いロックが検出されて実行できる

## 受け入れ基準
- [x] 状態ディレクトリ配下のロックを、書き込みと undo の区間に限って取得する
- [x] 標準ライブラリのみで実装する（`syscall.Flock` は darwin / linux で利用可。Windows は非対応と決定済み）
- [x] プロセス終了で自動解放される方式（flock）を第一候補とし、PID ファイル方式は採らない
- [x] 待機のタイムアウトを持つ

## テスト戦略
- E2E: 2 つのプロセスを同時起動して索引が壊れないこと
- 統合: ロック保持中の挙動
- 単体: ロックの取得・解放・タイムアウト

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` と `go test -race` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`cli.go` の `applyOrPreview` と `cmdUndo`、`internal/backup/backup.go` の `Record` / `Restore`（索引の読み→更新→保存）。

### 設計の確認
- 標準ライブラリの `syscall.Flock`（darwin / linux で利用可）で、状態ディレクトリのロックファイルを排他ロックする。プロセスが死ぬと OS が自動で解放する。PID ファイル方式は使わない。
- ロックを取るのは書き込み系（`--write` で実際に書く区間と `undo` の復元）だけ。プレビュー・`diff`・`status` は取らない。

### 手順
1. `internal/lock/lock.go`:

```go
// Package lock は状態ディレクトリの排他ロックを提供する。
package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Acquire は dir/lock を排他ロックし、解放関数を返す。
// timeout までポーリングし、取れなければエラーを返す。
func Acquire(dir string, timeout time.Duration) (func(), error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				f.Close()
			}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("別の provsync が実行中です (%s)", dir)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
```
2. `internal/lock/lock_test.go`: 1 回目の `Acquire` 成功 → 2 回目（timeout 200ms）が error → `release()` 後は成功。ロックファイルが残っていても保持されていなければ取得できる（異常終了の回復の代用テスト）。
3. `cli.go`: `applyOrPreview` で `o.write && p.Changed()` の分岐に入った直後（バックアップを記録する前）に `release, err := lock.Acquire(root.StateDir(), 10*time.Second)` を呼び、`defer release()`。`cmdUndo` も、復元を行う直前（`--list` 以外）で同様に取る。
4. 並行実行のテスト: 同じ root に対し 2 つの goroutine から `Run(... "push", "opencode", "--write")` を同時に呼び、どちらもエラーなく終わり、`undo --list` の索引 JSON が壊れていないこと。`go test -race` で実行する。

### 注意
- `syscall.Flock` は Windows に無いが、Windows は非対応と決定済み。ビルドタグは足さない（11 の `Supported` で弾く）。
- ロックを取る範囲を広げない。読み取り専用の経路は待たせない。
- ロックファイルの内容は使わない（空でよい）。
