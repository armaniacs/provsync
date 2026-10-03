// Package lock は状態ディレクトリの排他ロックを提供する。
package lock

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/armaniacs/provsync/internal/i18n"
)

// Acquire は dir/lock を排他ロックし、解放関数を返す。
// timeout までポーリングし、取れなければエラーを返す。
// flock はプロセス終了時に OS が解放するため、異常終了後の残存ロックも自動回復する。
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
			return nil, i18n.New("err.lock.busy", dir)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
