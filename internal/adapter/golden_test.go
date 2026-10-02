package adapter

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// TestPushGolden は各ツールの push 出力がゴールデンファイルとバイト単位で
// 一致することを確認する。更新は `go test -update ./internal/adapter` でのみ行い、
// 差分を必ず目で確認する。
func TestPushGolden(t *testing.T) {
	seed := `{
  "$schema": "x.schema.json",
  "theme": "dark",
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
			out, err := a.Push(providers)
			if err != nil {
				t.Fatal(err)
			}

			goldenPath := filepath.Join("testdata", name+"_push.golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("golden file missing (generate with -update): %v", err)
			}
			if !bytes.Equal(out, want) {
				t.Errorf("push output differs from golden:\n--- got\n%s\n--- want\n%s", out, want)
			}
		})
	}
}
