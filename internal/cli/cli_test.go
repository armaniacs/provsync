package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const kiloFixture = `{
  // kilocode config
  "$schema": "kilo.schema.json",
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1" },
      "models": { "m1": { "name": "m1" } },
      "reasoning": true
    },
    "llm-02": {
      "name": "LLM 02",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://b.example/v1" },
      "models": {}
    }
  }
}`

const opencodeFixture = `{
  "$schema": "opencode.schema.json",
  "provider": {
    "llm-02": {
      "name": "LLM 02 old",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://old.example/v1" },
      "models": {}
    },
    "legacy": {
      "name": "legacy",
      "npm": "pkg",
      "options": { "baseURL": "https://legacy.example/v1" },
      "models": {}
    }
  }
}`

type fixture struct {
	root     string
	kilo     string
	opencode string
	central  string
}

func setup(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{
		root:     root,
		kilo:     filepath.Join(root, ".config", "kilo", "kilo.jsonc"),
		opencode: filepath.Join(root, ".config", "opencode", "opencode.json"),
		central:  filepath.Join(root, ".config", "provsync", "config.json"),
	}
	write(t, f.kilo, kiloFixture)
	write(t, f.opencode, opencodeFixture)
	return f
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	full := append([]string{"--root", root}, args...)
	err := Run(full, &buf)
	return buf.String(), err
}

func mustRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := run(t, root, args...)
	if err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, out)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestList(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "list")
	for _, want := range []string{f.kilo, f.opencode, "未作成"} {
		if !strings.Contains(out, want) {
			t.Errorf("list missing %q:\n%s", want, out)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "--version")
	if !strings.HasPrefix(out, "provsync ") {
		t.Errorf("--version output must start with %q:\n%s", "provsync ", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("--version must be a single line:\n%s", out)
	}
}

func TestVersionSubcommand(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "version")
	if !strings.HasPrefix(out, "provsync ") {
		t.Errorf("version output must start with %q:\n%s", "provsync ", out)
	}
}

func TestNoArgsShowsVersion(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root)
	if !strings.Contains(out, "provsync ") {
		t.Errorf("no-args output must show the version:\n%s", out)
	}
}

func TestNoArgsShowsConfigPaths(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root)
	for _, want := range []string{f.central, f.kilo, f.opencode, "未作成", "バックアップ"} {
		if !strings.Contains(out, want) {
			t.Errorf("no-args output missing %q:\n%s", want, out)
		}
	}
}

func TestHelpFlagShowsConfigPaths(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "--help")
	for _, want := range []string{f.central, f.kilo, f.opencode, "未作成", "バックアップ"} {
		if !strings.Contains(out, want) {
			t.Errorf("--help output missing %q:\n%s", want, out)
		}
	}
}

func TestNoArgsAfterPullShowsCentralExists(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	out := mustRun(t, f.root)
	if strings.Contains(out, f.central+" (未作成)") {
		t.Errorf("central config must not be marked 未作成 after pull:\n%s", out)
	}
}

func TestDiffMasksSecrets(t *testing.T) {
	f := setup(t)
	write(t, f.opencode, `{
  "$schema": "opencode.schema.json",
  "provider": {
    "llm-02": {
      "name": "LLM 02 old",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://old.example/v1", "apiKey": "sk-test-SECRET-123" },
      "models": {}
    }
  }
}`)
	out := mustRun(t, f.root, "diff", "kilocode", "opencode")
	if strings.Contains(out, "sk-test-SECRET-123") {
		t.Errorf("diff output must not contain the raw secret:\n%s", out)
	}
	if !strings.Contains(out, "********") {
		t.Errorf("diff output must contain the mask:\n%s", out)
	}
}

func TestDiffShowSecrets(t *testing.T) {
	f := setup(t)
	write(t, f.opencode, `{
  "$schema": "opencode.schema.json",
  "provider": {
    "llm-02": {
      "name": "LLM 02 old",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://old.example/v1", "apiKey": "sk-test-SECRET-123" },
      "models": {}
    }
  }
}`)
	out := mustRun(t, f.root, "diff", "kilocode", "opencode", "--show-secrets")
	if !strings.Contains(out, "sk-test-SECRET-123") {
		t.Errorf("--show-secrets must show the raw secret:\n%s", out)
	}
	if !strings.Contains(out, "警告: --show-secrets") {
		t.Errorf("--show-secrets must print a warning:\n%s", out)
	}
}

func TestPushWritePreservesSecretValue(t *testing.T) {
	f := setup(t)
	write(t, f.kilo, `{
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1", "apiKey": "sk-KEEP-VALUE" },
      "models": {}
    }
  }
}`)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "kilocode", "--write")
	got := read(t, f.kilo)
	if !strings.Contains(got, "sk-KEEP-VALUE") {
		t.Errorf("push must preserve the raw secret value in the file:\n%s", got)
	}
	if strings.Contains(got, "********") {
		t.Errorf("written file must not contain the mask:\n%s", got)
	}
}

func TestPreviewPullDoesNotWrite(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "pull", "kilocode")
	if !strings.Contains(out, "プレビューのみ") {
		t.Errorf("expected preview notice:\n%s", out)
	}
	if _, err := os.Stat(f.central); !os.IsNotExist(err) {
		t.Error("preview must not create central config")
	}
}

func TestPullWriteCreatesCentral(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")

	var cfg map[string]any
	if err := json.Unmarshal([]byte(read(t, f.central)), &cfg); err != nil {
		t.Fatalf("central not valid JSON: %v", err)
	}
	providers := cfg["providers"].(map[string]any)
	if len(providers) != 2 {
		t.Fatalf("providers = %d, want 2", len(providers))
	}
	llm01 := providers["llm-01"].(map[string]any)
	if llm01["baseURL"] != "https://a.example/v1" {
		t.Errorf("baseURL = %v", llm01["baseURL"])
	}
}

func TestPushWriteUpdatesOpencodePreservingOthers(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "opencode", "--write")

	var doc map[string]any
	if err := json.Unmarshal([]byte(read(t, f.opencode)), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != "opencode.schema.json" {
		t.Error("non-provider key lost")
	}
	providers := doc["provider"].(map[string]any)
	if _, ok := providers["legacy"]; !ok {
		t.Error("unmanaged provider lost")
	}
	if _, ok := providers["llm-01"]; !ok {
		t.Error("llm-01 should be added")
	}
	llm02 := providers["llm-02"].(map[string]any)
	opts := llm02["options"].(map[string]any)
	if opts["baseURL"] != "https://b.example/v1" {
		t.Errorf("llm-02 baseURL = %v, want updated", opts["baseURL"])
	}
}

func TestPushIsIdempotent(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "opencode", "--write")
	before := read(t, f.opencode)

	out := mustRun(t, f.root, "push", "opencode")
	if !strings.Contains(out, "変更はありません") {
		t.Errorf("expected no changes:\n%s", out)
	}
	if read(t, f.opencode) != before {
		t.Error("preview push modified the file")
	}
}

func TestUndoRestoresPreviousState(t *testing.T) {
	f := setup(t)
	original := read(t, f.opencode)

	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "opencode", "--write")
	if read(t, f.opencode) == original {
		t.Fatal("push should have changed opencode")
	}

	mustRun(t, f.root, "undo")
	if got := read(t, f.opencode); got != original {
		t.Errorf("undo did not restore opencode\n got:\n%s\nwant:\n%s", got, original)
	}
}

func TestSyncWrite(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "sync", "--from", "kilocode", "--to", "opencode", "--write")

	if _, err := os.Stat(f.central); err != nil {
		t.Errorf("central not created: %v", err)
	}
	if !strings.Contains(read(t, f.opencode), "llm-01") {
		t.Error("opencode not updated by sync")
	}
}

func TestDiffDoesNotWrite(t *testing.T) {
	f := setup(t)
	original := read(t, f.opencode)
	out := mustRun(t, f.root, "diff", "kilocode", "opencode")

	if !strings.Contains(out, "--- a/") || !strings.Contains(out, "+++ b/") {
		t.Errorf("diff headers missing:\n%s", out)
	}
	if read(t, f.opencode) != original {
		t.Error("diff must not modify files")
	}
}

func TestStatusReports(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	out := mustRun(t, f.root, "status")
	if !strings.Contains(out, f.opencode) {
		t.Errorf("status missing tool path:\n%s", out)
	}
	if !strings.Contains(out, "llm-01") {
		t.Errorf("status missing drift info:\n%s", out)
	}
}

func TestProviderFilter(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--provider", "llm-01", "--write")
	var cfg map[string]any
	if err := json.Unmarshal([]byte(read(t, f.central)), &cfg); err != nil {
		t.Fatal(err)
	}
	providers := cfg["providers"].(map[string]any)
	if len(providers) != 1 {
		t.Errorf("providers = %d, want 1", len(providers))
	}
	if _, ok := providers["llm-01"]; !ok {
		t.Error("llm-01 missing")
	}
}

func TestPullPushRoundTripsUnknownFields(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "kilocode", "--write")

	var doc map[string]any
	if err := json.Unmarshal([]byte(read(t, f.kilo)), &doc); err != nil {
		t.Fatal(err)
	}
	entry := doc["provider"].(map[string]any)["llm-01"].(map[string]any)
	if entry["reasoning"] != true {
		t.Errorf("unknown field reasoning lost after round-trip: %v", entry)
	}
	if doc["$schema"] != "kilo.schema.json" {
		t.Error("non-provider key lost after round-trip")
	}
}

func TestUnknownToolErrors(t *testing.T) {
	f := setup(t)
	if _, err := run(t, f.root, "pull", "nope"); err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestPushWithoutCentralErrors(t *testing.T) {
	f := setup(t)
	if _, err := run(t, f.root, "push", "opencode"); err == nil {
		t.Error("expected error when central config is missing")
	}
}

func TestPullMissingToolConfigErrors(t *testing.T) {
	f := setup(t)
	if err := os.Remove(f.kilo); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, f.root, "pull", "kilocode"); err == nil {
		t.Error("expected error for missing tool config")
	}
}

func TestUndoListEmpty(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "undo", "--list")
	if !strings.Contains(out, "履歴はありません") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestProviderRepeatable(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--provider", "llm-01", "--provider", "llm-02", "--write")
	var cfg map[string]any
	if err := json.Unmarshal([]byte(read(t, f.central)), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := len(cfg["providers"].(map[string]any)); got != 2 {
		t.Errorf("providers = %d, want 2", got)
	}
}

func TestProviderFilterMissingErrors(t *testing.T) {
	f := setup(t)
	if _, err := run(t, f.root, "pull", "kilocode", "--provider", "nope"); err == nil {
		t.Error("expected error for unknown provider filter")
	}
}

func TestSyncPreviewDoesNotWrite(t *testing.T) {
	f := setup(t)
	original := read(t, f.opencode)
	out := mustRun(t, f.root, "sync", "--from", "kilocode", "--to", "opencode")
	if !strings.Contains(out, "プレビューのみ") {
		t.Errorf("expected preview notice:\n%s", out)
	}
	if _, err := os.Stat(f.central); !os.IsNotExist(err) {
		t.Error("sync preview must not create central config")
	}
	if read(t, f.opencode) != original {
		t.Error("sync preview must not modify opencode")
	}
}

func TestNoBackupRecordsMarker(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write", "--no-backup")

	idx := filepath.Join(f.root, ".local", "state", "provsync", "index.json")
	var manifest struct {
		Operations []struct {
			Command string `json:"command"`
			Files   []struct {
				Path string `json:"path"`
			} `json:"files"`
			NoBackup bool `json:"noBackup"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(read(t, idx)), &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(manifest.Operations) != 1 || !manifest.Operations[0].NoBackup {
		t.Fatalf("expected one marker operation: %+v", manifest.Operations)
	}
	if len(manifest.Operations[0].Files) != 0 {
		t.Errorf("marker must not hold backup files: %+v", manifest.Operations[0].Files)
	}
	backups := filepath.Join(f.root, ".local", "state", "provsync", "backups")
	if entries, err := os.ReadDir(backups); err == nil && len(entries) != 0 {
		t.Errorf("backup dirs created despite --no-backup: %d", len(entries))
	}
}

func TestPullWriteCreatesBackup(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	idx := filepath.Join(f.root, ".local", "state", "provsync", "index.json")
	var manifest struct {
		Operations []struct {
			Command string `json:"command"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(read(t, idx)), &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(manifest.Operations) == 0 || manifest.Operations[0].Command != "pull kilocode" {
		t.Errorf("unexpected manifest: %+v", manifest)
	}
}

func TestUndoByIDAndRedo(t *testing.T) {
	f := setup(t)
	original := read(t, f.opencode)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "opencode", "--write")
	pushed := read(t, f.opencode)

	// 明示 ID で undo
	opID := newestOperationID(t, f)
	mustRun(t, f.root, "undo", opID)
	if read(t, f.opencode) != original {
		t.Fatal("undo by id did not restore opencode")
	}

	// undo 自体を undo(redo)
	mustRun(t, f.root, "undo")
	if read(t, f.opencode) != pushed {
		t.Errorf("redo did not re-apply the pushed state")
	}
}

func newestOperationID(t *testing.T, f fixture) string {
	t.Helper()
	idx := filepath.Join(f.root, ".local", "state", "provsync", "index.json")
	var manifest struct {
		Operations []struct {
			ID string `json:"id"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(read(t, idx)), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Operations) == 0 {
		t.Fatal("no operations recorded")
	}
	return manifest.Operations[0].ID
}

func TestMalformedCentralErrors(t *testing.T) {
	f := setup(t)
	write(t, f.central, "{ not json")
	if _, err := run(t, f.root, "push", "opencode"); err == nil {
		t.Error("expected error for malformed central config")
	}
}

func TestMalformedToolConfigErrors(t *testing.T) {
	f := setup(t)
	write(t, f.kilo, "{ \"provider\": ")
	if _, err := run(t, f.root, "pull", "kilocode"); err == nil {
		t.Error("expected error for malformed tool config")
	}
}

func TestUnknownCommandErrors(t *testing.T) {
	f := setup(t)
	if _, err := run(t, f.root, "frobnicate"); err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestPushPreservesToolConfigSecrets(t *testing.T) {
	f := setup(t)
	write(t, f.kilo, `{
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1", "apiKey": "SECRET" },
      "models": {}
    }
  }
}`)

	out := mustRun(t, f.root, "pull", "kilocode", "--write")
	if !strings.Contains(out, "警告") {
		t.Errorf("pull should warn about the plaintext key:\n%s", out)
	}
	if strings.Contains(read(t, f.central), "SECRET") {
		t.Error("central config must not contain the secret")
	}

	mustRun(t, f.root, "push", "kilocode", "--write")
	if !strings.Contains(read(t, f.kilo), `"apiKey": "SECRET"`) {
		t.Error("push must preserve the existing options.apiKey in the tool config")
	}
}

func TestCrossToolPullPreservesExtras(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "pull", "opencode", "--write")
	mustRun(t, f.root, "push", "kilocode", "--write")

	if !strings.Contains(read(t, f.kilo), `"reasoning": true`) {
		t.Error("kilocode extras must survive pulling from another tool")
	}
}

func TestSecondPushPreviewReportsNoChanges(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "opencode", "--write")

	out := mustRun(t, f.root, "push", "opencode")
	if strings.Contains(out, "更新") {
		t.Errorf("second push preview must not report phantom updates:\n%s", out)
	}
	if !strings.Contains(out, "変更なし") {
		t.Errorf("expected 変更なし in preview:\n%s", out)
	}
}

func TestStatusNoFalseDriftAfterPush(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "opencode", "--write")

	out := mustRun(t, f.root, "status", "opencode")
	if strings.Contains(out, "差分あり") {
		t.Errorf("status must not report drift right after a successful push:\n%s", out)
	}
}

func TestStatusCorruptCentralErrors(t *testing.T) {
	f := setup(t)
	write(t, f.central, "{ not json")
	if _, err := run(t, f.root, "status"); err == nil {
		t.Error("status must fail on a corrupt central config")
	}
}

func TestPullPreservesCentralVersion(t *testing.T) {
	f := setup(t)
	write(t, f.central, `{"version": 2, "providers": {}}`)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	if !strings.Contains(read(t, f.central), `"version": 2`) {
		t.Errorf("pull must preserve the central config version:\n%s", read(t, f.central))
	}
}

func TestUndoWarnsAfterNoBackupWrite(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "opencode", "--write", "--no-backup")

	out := mustRun(t, f.root, "undo")
	if !strings.Contains(out, "バックアップなし") {
		t.Errorf("undo must warn about the unbacked write:\n%s", out)
	}
}
