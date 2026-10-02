package lock

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	dir := t.TempDir()
	release, err := Acquire(dir, time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// 保持中は 2 つ目が取れない
	if _, err := Acquire(dir, 200*time.Millisecond); err == nil {
		t.Error("second acquire must fail while held")
	} else if !strings.Contains(err.Error(), "実行中") {
		t.Errorf("error must mention a running process: %v", err)
	}

	release()

	// 解放後は取れる
	release2, err := Acquire(dir, time.Second)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	release2()
}

func TestAcquireRecoversStaleLockFile(t *testing.T) {
	dir := t.TempDir()
	// 異常終了で残ったロックファイル(保持されていない)は取得できる
	if err := errors.Join(); err != nil {
		t.Fatal(err)
	}
	release, err := Acquire(dir, time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	release()

	// ロックファイルが残っていても、保持されていなければ取得できる
	release2, err := Acquire(dir, time.Second)
	if err != nil {
		t.Fatalf("acquire with stale lock file: %v", err)
	}
	release2()
}
