package adapter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdapterContract は全アダプタが満たすべき契約を検証する。
// 新しいツールのアダプタは Names() に追加するだけでこの契約テストの対象になる。
// 契約:
//   - pull → push の出力は冪等(同じ入力から同じバイト列)
//   - ファイル内の秘密キーの値は保持される(削除しない)
//   - ツール固有の未知フィールドは失われない
//   - 秘密の値がカノニカル形へ漏れ出ない
func TestAdapterContract(t *testing.T) {
	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			a, err := Get(name, Root{ConfigHome: dir, StateHome: dir})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(a.Path()), 0o755); err != nil {
				t.Fatal(err)
			}
			seed := `{"provider":{"p1":{"name":"P1","npm":"pkg","options":{"baseURL":"https://a/v1","apiKey":"sk-test-KEEP"},"models":{"m":{"name":"m"}},"custom":true}}}`
			if err := os.WriteFile(a.Path(), []byte(seed), 0o644); err != nil {
				t.Fatal(err)
			}

			providers, _, err := a.Pull()
			if err != nil {
				t.Fatal(err)
			}
			out1, err := a.Push(providers)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(a.Path(), out1, 0o644); err != nil {
				t.Fatal(err)
			}
			out2, err := a.Push(providers)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out1, out2) {
				t.Error("push is not idempotent")
			}
			if !strings.Contains(string(out1), "sk-test-KEEP") {
				t.Error("in-file secret was dropped")
			}
			if !strings.Contains(string(out1), `"custom"`) {
				t.Error("tool-specific field was lost")
			}
			for _, p := range providers {
				if strings.Contains(fmt.Sprint(p), "sk-test-KEEP") {
					t.Error("secret leaked into canonical form")
				}
			}
		})
	}
}
