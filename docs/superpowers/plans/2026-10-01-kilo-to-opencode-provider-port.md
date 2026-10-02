# kilo → opencode Provider Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `~/.config/kilo/kilo.jsonc` の provider エントリ `llm-01` / `llm-02` / `llm-03` を `~/.config/opencode/opencode.json` へ移植する Go 製 CLI `provsync` を作る。既定はプレビュー、`--write` でバックアップ付きアトミック書き込み。

**Architecture:** 変換ロジックは `internal/syncer` に純粋関数として置き、`map[string]any` で provider エントリを忠実にパススルーする。CLI とファイル I/O は `main.go`。source は JSONC なので前処理で純 JSON 化してから `encoding/json` に渡す。

**Tech Stack:** Go 1.22、標準ライブラリのみ(`flag` / `encoding/json` / `os` / `path/filepath` / `testing`)。

参照スペック: `docs/superpowers/specs/2026-10-01-kilo-to-opencode-provider-port-design.md`

---

## File Structure

| ファイル | 責務 |
|---|---|
| `go.mod` | モジュール定義(`module provsync`, go 1.22) |
| `internal/syncer/jsonc.go` | JSONC(行コメント・末尾カンマ)を純 JSON に前処理 |
| `internal/syncer/jsonc_test.go` | `StripJSONC` のテスト |
| `internal/syncer/syncer.go` | `Merge`: source の provider を target へ上書き移植(純粋関数) |
| `internal/syncer/syncer_test.go` | `Merge` のテスト |
| `main.go` | フラグ解析、ファイル I/O、バックアップ、アトミック書き込み、プレビュー |
| `main_test.go` | `run()` の統合テスト(プレビュー不変 / --write / エラー) |

---

### Task 1: Go モジュール初期化

**Files:**
- Create: `go.mod`

- [ ] **Step 1: モジュールを初期化する**

Run:
```bash
go mod init provsync
```
Expected: `go: creating new go.mod: module provsync`

- [ ] **Step 2: go.mod を確認する**

`go.mod` が次の内容になっていること(Go バージョンはローカルの `go version` に合わせてよい):

```
module provsync

go 1.22
```

- [ ] **Step 3: Commit**

```bash
git add go.mod
git commit -m "chore: initialize go module"
```

---

### Task 2: JSONC 前処理 (StripJSONC)

**Files:**
- Create: `internal/syncer/jsonc.go`
- Test: `internal/syncer/jsonc_test.go`

- [ ] **Step 1: 失敗するテストを書く**

Create `internal/syncer/jsonc_test.go`:

```go
package syncer

import (
	"encoding/json"
	"testing"
)

func TestStripJSONC(t *testing.T) {
	in := []byte("{\n" +
		"  // line comment\n" +
		"  \"a\": 1, // trailing comment\n" +
		"  \"b\": [1, 2,],\n" +
		"  \"c\": \"// not a comment\",\n" +
		"  \"d\": \"brace,} inside string\",\n" +
		"}\n")

	var got map[string]any
	if err := json.Unmarshal(StripJSONC(in), &got); err != nil {
		t.Fatalf("StripJSONC produced invalid JSON: %v", err)
	}
	if got["a"].(float64) != 1 {
		t.Errorf("a = %v, want 1", got["a"])
	}
	if got["c"] != "// not a comment" {
		t.Errorf("c = %v, want %q", got["c"], "// not a comment")
	}
	if got["d"] != "brace,} inside string" {
		t.Errorf("d = %v, want %q", got["d"], "brace,} inside string")
	}
	b := got["b"].([]any)
	if len(b) != 2 {
		t.Errorf("b len = %d, want 2", len(b))
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認する**

Run: `go test ./internal/syncer/ -run TestStripJSONC -v`
Expected: FAIL(コンパイルエラー: `undefined: StripJSONC`)

- [ ] **Step 3: 最小実装を書く**

Create `internal/syncer/jsonc.go`:

```go
package syncer

// StripJSONC removes // line comments and trailing commas from a JSONC
// document so it can be parsed by encoding/json. Comment markers and commas
// inside string literals are preserved.
func StripJSONC(b []byte) []byte {
	return removeTrailingCommas(stripLineComments(b))
}

func stripLineComments(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inString := false
	escaped := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '/' {
			for i < len(b) && b[i] != '\n' {
				i++
			}
			if i < len(b) {
				out = append(out, b[i])
			}
			continue
		}
		out = append(out, c)
	}
	return out
}

func removeTrailingCommas(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inString := false
	escaped := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
				j++
			}
			if j < len(b) && (b[j] == '}' || b[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}
```

- [ ] **Step 4: テストを実行して成功を確認する**

Run: `go test ./internal/syncer/ -run TestStripJSONC -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/syncer/jsonc.go internal/syncer/jsonc_test.go
git commit -m "feat: add JSONC preprocessing for kilo config"
```

---

### Task 3: Provider マージ (Merge)

**Files:**
- Create: `internal/syncer/syncer.go`
- Test: `internal/syncer/syncer_test.go`

- [ ] **Step 1: 失敗するテストを書く**

Create `internal/syncer/syncer_test.go`:

```go
package syncer

import "testing"

func TestMergeOverwritesAndKeepsOthers(t *testing.T) {
	source := map[string]any{
		"provider": map[string]any{
			"llm-03": map[string]any{
				"npm":    "@ai-sdk/openai-compatible",
				"models": map[string]any{"new": map[string]any{"name": "new"}},
			},
			"llm-02": map[string]any{
				"models": map[string]any{"s": map[string]any{"name": "s"}},
			},
			"llm-01": map[string]any{
				"models": map[string]any{"i": map[string]any{"name": "i"}},
			},
		},
	}
	target := map[string]any{
		"provider": map[string]any{
			"llm-04": map[string]any{"models": map[string]any{}},
			"llm-02":  map[string]any{"models": map[string]any{"stale": map[string]any{}}},
		},
	}

	got, err := Merge(source, target, []string{"llm-01", "llm-02", "llm-03"})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	providers := got["provider"].(map[string]any)

	if _, ok := providers["llm-04"]; !ok {
		t.Error("llm-04 should be preserved")
	}
	if _, ok := providers["llm-01"]; !ok {
		t.Error("llm-01 should be added")
	}
	llm03 := providers["llm-03"].(map[string]any)
	if llm03["npm"] != "@ai-sdk/openai-compatible" {
		t.Errorf("llm03.npm = %v", llm03["npm"])
	}
	llm02 := providers["llm-02"].(map[string]any)
	models := llm02["models"].(map[string]any)
	if _, ok := models["stale"]; ok {
		t.Error("llm-02 should be overwritten, stale model still present")
	}
	if _, ok := models["s"]; !ok {
		t.Error("llm-02 model s missing")
	}
}

func TestMergeMissingProviderErrors(t *testing.T) {
	source := map[string]any{"provider": map[string]any{}}
	target := map[string]any{"provider": map[string]any{}}
	if _, err := Merge(source, target, []string{"llm-03"}); err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestMergeCreatesProviderSectionWhenAbsent(t *testing.T) {
	source := map[string]any{
		"provider": map[string]any{
			"llm-03": map[string]any{"models": map[string]any{}},
		},
	}
	got, err := Merge(source, map[string]any{}, []string{"llm-03"})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	providers, ok := got["provider"].(map[string]any)
	if !ok {
		t.Fatal("provider section not created")
	}
	if _, ok := providers["llm-03"]; !ok {
		t.Error("llm-03 not added")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認する**

Run: `go test ./internal/syncer/ -run TestMerge -v`
Expected: FAIL(コンパイルエラー: `undefined: Merge`)

- [ ] **Step 3: 最小実装を書く**

Create `internal/syncer/syncer.go`:

```go
package syncer

import "fmt"

// Merge copies the provider entries named by keys from source into target,
// overwriting entries with the same key. Provider entries not named in keys
// are preserved as-is. Returns an error if a requested key is absent from
// source.
func Merge(source, target map[string]any, keys []string) (map[string]any, error) {
	srcProviders, err := providerSection(source)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	tgtProviders, err := providerSection(target)
	if err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}
	for _, k := range keys {
		entry, ok := srcProviders[k]
		if !ok {
			return nil, fmt.Errorf("provider %q not found in source", k)
		}
		tgtProviders[k] = entry
	}
	target["provider"] = tgtProviders
	return target, nil
}

func providerSection(cfg map[string]any) (map[string]any, error) {
	v, ok := cfg["provider"]
	if !ok {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider is not an object")
	}
	return m, nil
}
```

- [ ] **Step 4: テストを実行して成功を確認する**

Run: `go test ./internal/syncer/ -v`
Expected: PASS(すべての syncer テスト)

- [ ] **Step 5: Commit**

```bash
git add internal/syncer/syncer.go internal/syncer/syncer_test.go
git commit -m "feat: add provider merge logic"
```

---

### Task 4: CLI とファイル I/O

**Files:**
- Create: `main.go`
- Modify: `main_test.go`(Task 5 で作成)

- [ ] **Step 1: main.go を実装する**

Create `main.go`:

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"provsync/internal/syncer"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "provsync:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("provsync", flag.ContinueOnError)
	fs.SetOutput(out)

	source := fs.String("source", filepath.Join(home, ".config", "kilo", "kilo.jsonc"), "source kilo config (JSONC)")
	target := fs.String("target", filepath.Join(home, ".config", "opencode", "opencode.json"), "target opencode config (JSON)")
	write := fs.Bool("write", false, "write changes to target (default: preview only)")
	backup := fs.Bool("backup", true, "create .bak before writing")
	providers := fs.String("providers", "llm-01,llm-02,llm-03", "comma-separated provider keys to port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	keys := splitKeys(*providers)
	if len(keys) == 0 {
		return fmt.Errorf("no providers specified")
	}

	srcBytes, err := os.ReadFile(*source)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	var src map[string]any
	if err := json.Unmarshal(syncer.StripJSONC(srcBytes), &src); err != nil {
		return fmt.Errorf("parse source: %w", err)
	}

	tgtBytes, err := os.ReadFile(*target)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}
	var tgt map[string]any
	if err := json.Unmarshal(tgtBytes, &tgt); err != nil {
		return fmt.Errorf("parse target: %w", err)
	}

	merged, err := syncer.Merge(src, tgt, keys)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	for _, k := range keys {
		fmt.Fprintf(out, "%s: %s\n", k, summarize(merged, k))
	}

	if !*write {
		fmt.Fprintln(out, "(preview only; use --write to apply)")
		return nil
	}

	if *backup {
		if err := copyFile(*target, *target+".bak"); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	if err := atomicWrite(*target, data); err != nil {
		return fmt.Errorf("write target: %w", err)
	}
	fmt.Fprintf(out, "wrote %s\n", *target)
	return nil
}

func summarize(cfg map[string]any, key string) string {
	providers, _ := cfg["provider"].(map[string]any)
	entry, _ := providers[key].(map[string]any)
	models, _ := entry["models"].(map[string]any)
	return fmt.Sprintf("置換 (models %d件)", len(models))
}

func splitKeys(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".provsync-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
```

- [ ] **Step 2: ビルドとフォーマットを確認する**

Run: `gofmt -l . && go build ./...`
Expected: `gofmt -l` が何も出力しない。`go build ./...` が成功する。

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: add provsync CLI with preview and atomic write"
```

---

### Task 5: run() 統合テスト

**Files:**
- Create: `main_test.go`

- [ ] **Step 1: 失敗するテストを書く**

Create `main_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFixtures(t *testing.T) (source, target string, origTarget []byte) {
	t.Helper()
	dir := t.TempDir()
	source = filepath.Join(dir, "kilo.jsonc")
	target = filepath.Join(dir, "opencode.json")

	src := "{\n" +
		"  // source config\n" +
		"  \"provider\": {\n" +
		"    \"llm-03\": { \"npm\": \"@ai-sdk/openai-compatible\", \"models\": { \"a\": { \"name\": \"a\" } } },\n" +
		"    \"llm-02\": { \"models\": { \"s\": { \"name\": \"s\" } } },\n" +
		"    \"llm-01\": { \"models\": { \"i\": { \"name\": \"i\" } } },\n" +
		"  },\n" +
		"}\n"
	if err := os.WriteFile(source, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	origTarget = []byte("{\"provider\":{\"llm-02\":{\"models\":{\"stale\":{}}},\"llm-04\":{\"models\":{}}}}")
	if err := os.WriteFile(target, origTarget, 0o644); err != nil {
		t.Fatal(err)
	}
	return source, target, origTarget
}

func TestRunPreviewDoesNotWrite(t *testing.T) {
	source, target, orig := writeFixtures(t)

	var out bytes.Buffer
	if err := run([]string{"--source", source, "--target", target}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, orig) {
		t.Error("preview must not modify target")
	}
	if _, err := os.Stat(target + ".bak"); err == nil {
		t.Error("preview must not create backup")
	}
}

func TestRunWriteMergesAndBacksUp(t *testing.T) {
	source, target, orig := writeFixtures(t)

	var out bytes.Buffer
	if err := run([]string{"--source", source, "--target", target, "--write"}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	bak, err := os.ReadFile(target + ".bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if !bytes.Equal(bak, orig) {
		t.Error("backup content does not match original")
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("target is not valid JSON: %v", err)
	}
	providers := cfg["provider"].(map[string]any)
	if _, ok := providers["llm-03"]; !ok {
		t.Error("llm-03 missing")
	}
	if _, ok := providers["llm-01"]; !ok {
		t.Error("llm-01 missing")
	}
	if _, ok := providers["llm-04"]; !ok {
		t.Error("llm-04 should be preserved")
	}
	llm02 := providers["llm-02"].(map[string]any)
	models := llm02["models"].(map[string]any)
	if _, ok := models["stale"]; ok {
		t.Error("llm-02 should be overwritten")
	}
	if _, ok := models["s"]; !ok {
		t.Error("llm-02 model s missing")
	}
}

func TestRunNoBackupWhenDisabled(t *testing.T) {
	source, target, _ := writeFixtures(t)

	var out bytes.Buffer
	if err := run([]string{"--source", source, "--target", target, "--write", "--backup=false"}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(target + ".bak"); err == nil {
		t.Error("backup should be disabled")
	}
}

func TestRunMissingProviderFails(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "kilo.jsonc")
	target := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(source, []byte(`{"provider":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`{"provider":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"--source", source, "--target", target}, &out); err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestRunMissingFileFails(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := run([]string{
		"--source", filepath.Join(dir, "nope.jsonc"),
		"--target", filepath.Join(dir, "nope.json"),
	}, &out)
	if err == nil {
		t.Error("expected error for missing source file")
	}
}
```

- [ ] **Step 2: テストを実行する**

Run: `go test ./... -v`
Expected: PASS(全テスト)

- [ ] **Step 3: 手動スモークテスト(実ファイルでプレビュー)**

Run:
```bash
go run . --write=false
```
Expected: `llm-01:` `llm-02:` `llm-03:` の要約行と `(preview only; use --write to apply)` が表示され、`~/.config/opencode/opencode.json` は変更されない。`git status` や `diff` で確認する。

- [ ] **Step 4: フォーマットと vet**

Run: `gofmt -l . && go vet ./...`
Expected: 出力なし。

- [ ] **Step 5: Commit**

```bash
git add main_test.go
git commit -m "test: add integration tests for provsync run"
```

---

## Self-Review

**Spec coverage:**
- 目的(3 provider の移植)→ Task 3 `Merge` + Task 4 CLI(既定 `llm-01,llm-02,llm-03`)。
- 既定プレビュー / `--write` → Task 4 実装、Task 5 `TestRunPreviewDoesNotWrite`。
- バックアップ + アトミック書き込み → Task 4 `copyFile` / `atomicWrite`、Task 5 `TestRunWriteMergesAndBacksUp`。
- JSONC 前処理 → Task 2。
- 任意フィールド保持・型付き struct 不採用 → Task 3 は `map[string]any` をそのまま代入。
- target の他キー保持(`llm-04` 等)→ Task 3 / Task 5。
- 欠落時エラー → Task 3 `TestMergeMissingProviderErrors`、Task 5 `TestRunMissingProviderFails`。
- `$HOME` 基準の既定パス → Task 4 `os.UserHomeDir()` + 既定値。
- 全体再整形(2 スペース)→ Task 4 `json.MarshalIndent(merged, "", "  ")`。

**Placeholder scan:** "TBD"/"TODO"/"適切に処理" の類なし。全ステップに完全なコードまたはコマンドを記載。

**Type consistency:** `StripJSONC([]byte) []byte`、`Merge(map[string]any, map[string]any, []string) (map[string]any, error)`、`run([]string, io.Writer) error` は全タスクで一貫。`providerSection` は syncer.go 内のみで使用。