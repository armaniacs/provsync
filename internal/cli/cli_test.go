package cli

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestInitWriteCreatesCentral(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "init", "kilocode", "--write")
	if _, err := os.Stat(f.central); err != nil {
		t.Fatalf("init must create the central config: %v\n%s", err, out)
	}
	if !strings.Contains(out, "次の手順") {
		t.Errorf("init must print next steps:\n%s", out)
	}
}

func TestInitPreviewDoesNotWrite(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "init", "kilocode")
	if _, err := os.Stat(f.central); !os.IsNotExist(err) {
		t.Error("init preview must not create the central config")
	}
	if !strings.Contains(out, "--write") {
		t.Errorf("init preview must guide to --write:\n%s", out)
	}
}

func TestInitDetectsSingleTool(t *testing.T) {
	f := setup(t)
	if err := os.Remove(f.opencode); err != nil {
		t.Fatal(err)
	}
	out := mustRun(t, f.root, "init", "--write")
	if _, err := os.Stat(f.central); err != nil {
		t.Fatalf("init must create the central config: %v\n%s", err, out)
	}
	if !strings.Contains(out, "検出したツール: kilocode") {
		t.Errorf("init must report the detected tool:\n%s", out)
	}
}

func TestInitMultipleToolsListsCandidates(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "init")
	if _, err := os.Stat(f.central); !os.IsNotExist(err) {
		t.Error("init with multiple candidates must not create the central config")
	}
	if !strings.Contains(out, "kilocode") || !strings.Contains(out, "opencode") {
		t.Errorf("init must list detected tools:\n%s", out)
	}
	if !strings.Contains(out, "provsync init <tool>") {
		t.Errorf("init must guide to specify the tool:\n%s", out)
	}
}

func TestInitAlreadyInitializedErrors(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	before := read(t, f.central)

	_, err := run(t, f.root, "init", "kilocode", "--write")
	if err == nil {
		t.Error("init must error when already initialized")
	}
	if !strings.Contains(err.Error(), "pull") {
		t.Errorf("init error must guide to pull:\n%v", err)
	}
	if read(t, f.central) != before {
		t.Error("init must not modify the existing central config")
	}
}

func TestInitNoToolsErrors(t *testing.T) {
	f := setup(t)
	if err := os.Remove(f.kilo); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.opencode); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, f.root, "init")
	if err == nil {
		t.Error("init must error when no tool config exists")
	}
	if !strings.Contains(err.Error(), f.kilo) || !strings.Contains(err.Error(), f.opencode) {
		t.Errorf("init error must list searched paths:\n%v", err)
	}
}

func TestInitSecretWarning(t *testing.T) {
	f := setup(t)
	write(t, f.kilo, `{
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1", "apiKey": "sk-test-1" },
      "models": {}
    }
  }
}`)
	out := mustRun(t, f.root, "init", "kilocode", "--write")
	if strings.Contains(read(t, f.central), "sk-test-1") {
		t.Error("central config must not contain the secret")
	}
	if !strings.Contains(out, "apiKeyEnv") {
		t.Errorf("init must warn about apiKeyEnv:\n%s", out)
	}
}

func TestKeepEnvLimitsRetention(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_KEEP", "3")
	for i := 0; i < 4; i++ {
		write(t, f.central, `{"providers": {}}`)
		mustRun(t, f.root, "pull", "kilocode", "--write")
	}
	out := mustRun(t, f.root, "undo", "--list")
	if got := strings.Count(out, "pull kilocode"); got != 3 {
		t.Errorf("undo --list must show 3 ops with PROVSYNC_KEEP=3:\n%s", out)
	}
}

func TestKeepEnvInvalidErrors(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_KEEP", "abc")
	_, err := run(t, f.root, "pull", "kilocode", "--write")
	if err == nil {
		t.Error("expected error for invalid PROVSYNC_KEEP")
	}
	if !strings.Contains(err.Error(), "PROVSYNC_KEEP") {
		t.Errorf("error must mention PROVSYNC_KEEP: %v", err)
	}
}

func TestUndoPrune(t *testing.T) {
	f := setup(t)
	for i := 0; i < 4; i++ {
		write(t, f.central, `{"providers": {}}`)
		mustRun(t, f.root, "pull", "kilocode", "--write")
	}
	out := mustRun(t, f.root, "undo", "--prune", "--keep", "1")
	if !strings.Contains(out, "削除: 3 件") {
		t.Errorf("undo --prune must report removed count:\n%s", out)
	}
	out = mustRun(t, f.root, "undo", "--list")
	if got := strings.Count(out, "pull kilocode"); got != 1 {
		t.Errorf("undo --list must show 1 op after prune:\n%s", out)
	}
}

func TestStatusWarnsLoosePerm(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	if err := os.Chmod(f.central, 0o644); err != nil {
		t.Fatal(err)
	}
	out := mustRun(t, f.root, "status")
	if !strings.Contains(out, "権限が緩い") {
		t.Errorf("status must warn about loose permissions:\n%s", out)
	}
	if !strings.Contains(out, "chmod 600") {
		t.Errorf("status must suggest chmod 600:\n%s", out)
	}
}

func TestInitCreatesCentralWithStrictPerms(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "init", "kilocode", "--write")
	info, err := os.Stat(f.central)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("central mode = %o, want 600", info.Mode().Perm())
	}
}

func TestPushKeepsSymlink(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	real := filepath.Join(t.TempDir(), "opencode.json")
	data, _ := os.ReadFile(f.opencode)
	if err := os.WriteFile(real, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.opencode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, f.opencode); err != nil {
		t.Fatal(err)
	}

	mustRun(t, f.root, "push", "opencode", "--write")

	info, err := os.Lstat(f.opencode)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink was replaced by a regular file")
	}
	got, _ := os.ReadFile(real)
	if !strings.Contains(string(got), "llm-01") {
		t.Error("link target was not updated")
	}
}

func TestUndoKeepsSymlink(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	real := filepath.Join(t.TempDir(), "opencode.json")
	original, _ := os.ReadFile(f.opencode)
	if err := os.WriteFile(real, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.opencode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, f.opencode); err != nil {
		t.Fatal(err)
	}

	mustRun(t, f.root, "push", "opencode", "--write")
	mustRun(t, f.root, "undo")

	info, err := os.Lstat(f.opencode)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("undo replaced the symlink with a regular file")
	}
	got, _ := os.ReadFile(real)
	if !strings.Contains(string(got), "LLM 02 old") {
		t.Errorf("link target was not restored to the original content:\n%s", got)
	}
}

func TestPushBrokenSymlinkErrors(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	if err := os.Remove(f.opencode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing.json"), f.opencode); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, f.root, "push", "opencode", "--write")
	if err == nil {
		t.Error("expected error for broken symlink")
	}
	if !strings.Contains(err.Error(), "リンク先") {
		t.Errorf("error must mention the broken symlink: %v", err)
	}
}

func TestStatusJSON(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	out := mustRun(t, f.root, "status", "--json")

	var rep struct {
		SchemaVersion int `json:"schemaVersion"`
		Central       struct {
			Path      string `json:"path"`
			Exists    bool   `json:"exists"`
			Providers int    `json:"providers"`
		} `json:"central"`
		Tools []struct {
			Name  string   `json:"name"`
			Drift []string `json:"drift"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("status --json is not valid JSON: %v\n%s", err, out)
	}
	if rep.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", rep.SchemaVersion)
	}
	if !rep.Central.Exists || rep.Central.Providers != 2 {
		t.Errorf("central = %+v", rep.Central)
	}
	if len(rep.Tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(rep.Tools))
	}
}

func TestStatusExitCodeNoDrift(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	mustRun(t, f.root, "push", "kilocode", "--write")
	if _, err := run(t, f.root, "status", "kilocode", "--exit-code"); err != nil {
		t.Errorf("synced tool must exit 0 with --exit-code: %v", err)
	}
}

func TestStatusExitCodeWithDrift(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	_, err := run(t, f.root, "status", "opencode", "--exit-code")
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 3 {
		t.Errorf("drift must exit with ExitError{3}, got %T: %v", err, err)
	}
}

func TestListJSON(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "list", "--json")
	var rep struct {
		SchemaVersion int `json:"schemaVersion"`
		Tools         []struct {
			Name   string `json:"name"`
			Exists bool   `json:"exists"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("list --json is not valid JSON: %v\n%s", err, out)
	}
	if rep.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", rep.SchemaVersion)
	}
	if len(rep.Tools) != 2 || !rep.Tools[0].Exists {
		t.Errorf("tools = %+v", rep.Tools)
	}
}

func TestDiffJSONMasksSecrets(t *testing.T) {
	f := setup(t)
	write(t, f.opencode, `{
  "$schema": "opencode.schema.json",
  "provider": {
    "llm-02": {
      "name": "LLM 02 old",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://old.example/v1", "apiKey": "sk-test-SECRET-999" },
      "models": {}
    }
  }
}`)
	out := mustRun(t, f.root, "diff", "kilocode", "opencode", "--json")
	if strings.Contains(out, "sk-test-SECRET-999") {
		t.Errorf("diff --json must mask secrets:\n%s", out)
	}
	var rep struct {
		SchemaVersion int `json:"schemaVersion"`
		Changes       []struct {
			Tool string `json:"tool"`
			Path string `json:"path"`
			Diff string `json:"diff"`
		} `json:"changes"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("diff --json is not valid JSON: %v\n%s", err, out)
	}
}

const aliasCentral = `{
  "aliases": {
    "sonnet": {
      "kilocode": "claude-sonnet-4-5",
      "opencode": "anthropic/claude-sonnet-4-5"
    }
  },
  "providers": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "baseURL": "https://a.example/v1",
      "models": {
        "sonnet": { "name": "Sonnet" }
      }
    }
  },
  "version": 1
}`

func TestPushResolvesAliases(t *testing.T) {
	f := setup(t)
	write(t, f.central, aliasCentral)

	mustRun(t, f.root, "push", "opencode", "--write")
	got := read(t, f.opencode)
	if !strings.Contains(got, "anthropic/claude-sonnet-4-5") {
		t.Errorf("push must write the opencode ID for the alias:\n%s", got)
	}
	if strings.Contains(got, `"sonnet": {`) {
		t.Errorf("the alias key must be replaced in the tool config:\n%s", got)
	}

	mustRun(t, f.root, "push", "kilocode", "--write")
	gotKilo := read(t, f.kilo)
	if !strings.Contains(gotKilo, "claude-sonnet-4-5") || strings.Contains(gotKilo, "anthropic/claude-sonnet-4-5") {
		t.Errorf("kilocode must use its own ID:\n%s", gotKilo)
	}
}

func TestPushUndefinedAliasWarnsAndPassesThrough(t *testing.T) {
	f := setup(t)
	write(t, f.central, `{
  "aliases": {
    "sonnet": { "kilocode": "claude-sonnet-4-5" }
  },
  "providers": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "baseURL": "https://a.example/v1",
      "models": { "sonnet": { "name": "Sonnet" } }
    }
  },
  "version": 1
}`)

	out := mustRun(t, f.root, "push", "opencode", "--write")
	if !strings.Contains(out, "警告") {
		t.Errorf("undefined alias must warn:\n%s", out)
	}
	if !strings.Contains(read(t, f.opencode), `"sonnet"`) {
		t.Error("undefined alias must pass the key through as-is")
	}
}

func TestPushStrictErrorsOnUndefinedAlias(t *testing.T) {
	f := setup(t)
	write(t, f.central, `{
  "aliases": {
    "sonnet": { "kilocode": "claude-sonnet-4-5" }
  },
  "providers": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "baseURL": "https://a.example/v1",
      "models": { "sonnet": { "name": "Sonnet" } }
    }
  },
  "version": 1
}`)

	_, err := run(t, f.root, "push", "opencode", "--strict", "--write")
	if err == nil {
		t.Error("--strict must error on undefined aliases")
	}
	if strings.Contains(read(t, f.opencode), "sonnet") {
		t.Error("--strict must not write anything")
	}
}

func TestPullPreservesAliases(t *testing.T) {
	f := setup(t)
	write(t, f.central, aliasCentral)

	mustRun(t, f.root, "pull", "kilocode", "--write")
	if !strings.Contains(read(t, f.central), `"aliases"`) {
		t.Error("pull must preserve the aliases section")
	}
}

func TestPullKeepsModelIDs(t *testing.T) {
	f := setup(t)
	write(t, f.kilo, `{
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1" },
      "models": { "anthropic/claude-sonnet-4-5": { "name": "Sonnet" } }
    }
  }
}`)

	mustRun(t, f.root, "pull", "kilocode", "--write")
	if !strings.Contains(read(t, f.central), "anthropic/claude-sonnet-4-5") {
		t.Error("pull must store the model ID as-is")
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

func TestUnknownCommandIsUsageError(t *testing.T) {
	f := setup(t)
	_, err := run(t, f.root, "frobnicate")
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Errorf("unknown command must be a *UsageError, got %T: %v", err, err)
	}
}

func TestBadArgsAreUsageError(t *testing.T) {
	f := setup(t)
	for _, args := range [][]string{{"pull"}, {"push"}, {"diff", "only-one"}, {"init", "a", "b"}} {
		_, err := run(t, f.root, args...)
		var ue *UsageError
		if !errors.As(err, &ue) {
			t.Errorf("args %v must be a *UsageError, got %T: %v", args, err, err)
		}
	}
}

func TestRuntimeErrorIsNotUsageError(t *testing.T) {
	f := setup(t)
	write(t, f.central, "{ not json")
	_, err := run(t, f.root, "push", "opencode")
	if err == nil {
		t.Fatal("expected error")
	}
	var ue *UsageError
	if errors.As(err, &ue) {
		t.Errorf("runtime error must not be a *UsageError: %v", err)
	}
}

func TestSubcommandHelp(t *testing.T) {
	f := setup(t)
	for _, cmd := range []string{"list", "status", "init", "pull", "push", "sync", "diff", "undo", "completion", "version"} {
		out := mustRun(t, f.root, cmd, "--help")
		if !strings.Contains(out, cmd) {
			t.Errorf("%s --help output missing %q:\n%s", cmd, cmd, out)
		}
	}
	if out := mustRun(t, f.root, "pull", "--help"); !strings.Contains(out, "--write") {
		t.Errorf("pull --help must mention --write:\n%s", out)
	}
}

func TestCompletionZsh(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "completion", "zsh")
	if !strings.Contains(out, "compdef") || !strings.Contains(out, "kilocode") {
		t.Errorf("completion zsh output:\n%s", out)
	}
}

func TestCompletionBash(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "completion", "bash")
	if !strings.Contains(out, "complete") || !strings.Contains(out, "opencode") {
		t.Errorf("completion bash output:\n%s", out)
	}
}

func TestCompletionFish(t *testing.T) {
	f := setup(t)
	out := mustRun(t, f.root, "completion", "fish")
	if !strings.Contains(out, "complete -c provsync") {
		t.Errorf("completion fish output:\n%s", out)
	}
}

func TestCompletionUnknownShellErrors(t *testing.T) {
	f := setup(t)
	if _, err := run(t, f.root, "completion", "powershell"); err == nil {
		t.Error("expected error for unknown shell")
	}
}

func TestWarningsGoToErrOut(t *testing.T) {
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

	var out, errOut bytes.Buffer
	if err := RunWith([]string{"--root", f.root, "pull", "kilocode", "--write"}, &out, &errOut); err != nil {
		t.Fatalf("RunWith: %v", err)
	}
	if strings.Contains(out.String(), "警告") {
		t.Errorf("warnings must not go to stdout:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "警告") {
		t.Errorf("warnings must go to stderr:\n%s", errOut.String())
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
