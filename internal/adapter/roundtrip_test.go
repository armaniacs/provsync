package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPushIsIdempotentPerTool は全アダプタについて pull → push の出力が
// 冪等(同じ入力から同じバイト列)であることを確認する。
func TestPushIsIdempotentPerTool(t *testing.T) {
	seed := `{
  "$schema": "x.schema.json",
  "provider": {
    "p1": {
      "name": "P1",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1", "apiKey": "sk-test-KEEP" },
      "models": { "m": { "name": "m" } },
      "reasoning": true
    }
  }
}`
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
			if string(out1) != string(out2) {
				t.Errorf("push is not idempotent:\n--- first\n%s\n--- second\n%s", out1, out2)
			}
		})
	}
}
