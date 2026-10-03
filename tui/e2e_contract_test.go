package main

// 実バイナリ相手の連携契約テスト(PBI 33)。
// fakeBin(シェルスタブ)相手の単体テストでは検出できない破損、
// すなわち TUI が組み立てる `push <tool> --provider <p> --write` の
// 引数体系が CLI 側とずれた場合の実行時失敗を固定する。
// 本番コード・go.mod には触れず、このファイルの追加のみで構成する。

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildRealProvsync は親モジュールの provsync バイナリをテスト固有の
// temp ディレクトリへビルドする。ビルド失敗はスキップではなく失敗扱い
// (契約検証の素通りを防ぐ)。並列衝突を避けるため出力先は t.TempDir()。
func buildRealProvsync(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller cannot locate this test file")
	}
	repoRoot := filepath.Dir(filepath.Dir(thisFile))
	dir := t.TempDir()
	bin := filepath.Join(dir, "provsync-e2e")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build real provsync: %v\n%s", err, out)
	}
	return bin
}

// isolateHomeEnv は実ホーム・実設定ファイルに触れないための env 隔離。
// 実バイナリは HOME/XDG からパス解決するため、temp HOME に差し替え、
// XDG 系は空にして HOME 配下に落とす。言語は en に pin して drift 行を安定させる。
func isolateHomeEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("PROVSYNC_LANG", "en")
	return home
}

func writeE2EFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// e2eSelectOnly は指定の組み合わせだけを選択状態にする。
// 中央に無い provider(not-in-central)を push すると実バイナリが
// エラーにするため、意図した drift だけを選ぶ。
func e2eSelectOnly(sel *selection, want []pair) {
	wantSet := map[pair]bool{}
	for _, p := range want {
		wantSet[p] = true
	}
	for _, p := range sel.Items {
		sel.Selected[p] = wantSet[p]
	}
}

// e2eApplyAll は applyArgs の全引数列を実バイナリ相手に実行する。
// 引数体系のずれはここで非 nil エラーとして検出される。
func e2eApplyAll(t *testing.T, bin string, argsList [][]string) {
	t.Helper()
	if len(argsList) == 0 {
		t.Fatal("applyArgs must produce at least one command")
	}
	for _, args := range argsList {
		if len(args) != 5 || args[0] != "push" || args[2] != "--provider" || args[4] != "--write" {
			t.Fatalf("contract broken: args = %q, want [push <tool> --provider <p> --write]", args)
		}
		stdout, stderr, err := runProvsync(bin, args...)
		if err != nil {
			t.Fatalf("runProvsync %q: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout, stderr)
		}
	}
}

// toolProviderBaseURLs はツール設定ファイルの provider → baseURL 対応を返す。
// push 後のファイルは素の JSON に再シリアライズされるため encoding/json で読める。
func toolProviderBaseURLs(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Provider map[string]struct {
			Options map[string]string `json:"options"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("tool config %s is not valid JSON: %v\n%s", path, err, raw)
	}
	out := make(map[string]string, len(doc.Provider))
	for name, e := range doc.Provider {
		out[name] = e.Options["baseURL"]
	}
	return out
}

// e2eDriftProviders は fetchStatus 結果から指定ツールの drift 中 provider 名集合を返す。
func e2eDriftProviders(rep statusReport, tool string) map[string]bool {
	out := map[string]bool{}
	for _, ts := range rep.Tools {
		if ts.Name != tool {
			continue
		}
		for _, e := range ts.DriftEntries {
			out[e.Provider] = true
		}
	}
	return out
}

// TestE2EContractKilocodePushWithRealBinary は BDD「実バイナリ相手に適用が通る」
// の kilocode(JSONC) 版。中央にだけある provider と値がずれた provider の
// 2 種の drift を全路で適用し、実ファイルが中央どおりになることを検証する。
func TestE2EContractKilocodePushWithRealBinary(t *testing.T) {
	home := isolateHomeEnv(t)
	t.Setenv("PROVSYNC_BIN", buildRealProvsync(t))
	bin := os.Getenv("PROVSYNC_BIN")

	kilo := filepath.Join(home, ".config", "kilo", "kilo.jsonc")
	central := filepath.Join(home, ".config", "provsync", "config.json")
	writeE2EFile(t, kilo, `// e2e fixture
{"provider": {"e2e-drift": {"name": "E2E Drift", "npm": "@ai-sdk/openai-compatible",
"options": {"baseURL": "https://old.example/v1"}, "models": {}}}}`)
	writeE2EFile(t, central, `{"version":1,"providers":{`+
		`"e2e-new": {"name": "E2E New", "npm": "@ai-sdk/openai-compatible",`+
		` "baseURL": "https://central.example/v1", "models": {}},`+
		`"e2e-drift": {"name": "E2E Drift", "npm": "@ai-sdk/openai-compatible",`+
		` "baseURL": "https://central.example/v1", "models": {}}}}`)

	rep, err := fetchStatus(bin)
	if err != nil {
		t.Fatalf("fetchStatus: %v", err)
	}
	sel := newSelection(rep)
	want := []pair{{Tool: "kilocode", Provider: "e2e-new"}, {Tool: "kilocode", Provider: "e2e-drift"}}
	for _, p := range want {
		found := false
		for _, item := range sel.Items {
			if item == p {
				found = true
			}
		}
		if !found {
			t.Fatalf("drift not detected for %v: items = %v", p, sel.Items)
		}
	}
	e2eSelectOnly(sel, want)
	var kilocodeArgs [][]string
	for _, args := range sel.applyArgs() {
		if args[1] != "kilocode" {
			t.Fatalf("unexpected tool in args: %q", args)
		}
		kilocodeArgs = append(kilocodeArgs, args)
	}
	e2eApplyAll(t, bin, kilocodeArgs)

	got := toolProviderBaseURLs(t, kilo)
	if got["e2e-new"] != "https://central.example/v1" {
		t.Errorf("e2e-new baseURL = %q, want the central value", got["e2e-new"])
	}
	if got["e2e-drift"] != "https://central.example/v1" {
		t.Errorf("e2e-drift baseURL = %q, want the central value", got["e2e-drift"])
	}

	after, err := fetchStatus(bin)
	if err != nil {
		t.Fatalf("fetchStatus after push: %v", err)
	}
	if drift := e2eDriftProviders(after, "kilocode"); len(drift) != 0 {
		t.Errorf("drift must be clean after push: %v", drift)
	}
}

// TestE2EContractOpencodePushWithRealBinary は BDD「実バイナリ相手に適用が通る」
// の opencode(素の JSON) 版。中央にだけある provider を全路で適用する。
func TestE2EContractOpencodePushWithRealBinary(t *testing.T) {
	home := isolateHomeEnv(t)
	t.Setenv("PROVSYNC_BIN", buildRealProvsync(t))
	bin := os.Getenv("PROVSYNC_BIN")

	opencode := filepath.Join(home, ".config", "opencode", "opencode.json")
	central := filepath.Join(home, ".config", "provsync", "config.json")
	writeE2EFile(t, opencode, `{"provider": {"e2e-keep": {"name": "keep",`+
		` "npm": "@ai-sdk/openai-compatible", "options": {"baseURL": "https://keep.example/v1"},`+
		` "models": {}}}}`)
	writeE2EFile(t, central, `{"version":1,"providers":{`+
		`"e2e-oc": {"name": "E2E OC", "npm": "@ai-sdk/openai-compatible",`+
		` "baseURL": "https://central.example/v1", "models": {}}}}`)

	rep, err := fetchStatus(bin)
	if err != nil {
		t.Fatalf("fetchStatus: %v", err)
	}
	sel := newSelection(rep)
	found := false
	for _, item := range sel.Items {
		if item == (pair{Tool: "opencode", Provider: "e2e-oc"}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("drift not detected for opencode/e2e-oc: items = %v", sel.Items)
	}
	e2eSelectOnly(sel, []pair{{Tool: "opencode", Provider: "e2e-oc"}})
	var ocArgs [][]string
	for _, args := range sel.applyArgs() {
		if args[1] != "opencode" {
			t.Fatalf("unexpected tool in args: %q", args)
		}
		ocArgs = append(ocArgs, args)
	}
	e2eApplyAll(t, bin, ocArgs)

	got := toolProviderBaseURLs(t, opencode)
	if got["e2e-oc"] != "https://central.example/v1" {
		t.Errorf("e2e-oc baseURL = %q, want the central value", got["e2e-oc"])
	}
	if got["e2e-keep"] != "https://keep.example/v1" {
		t.Errorf("e2e-keep must be preserved, got baseURL = %q", got["e2e-keep"])
	}

	after, err := fetchStatus(bin)
	if err != nil {
		t.Fatalf("fetchStatus after push: %v", err)
	}
	drift := e2eDriftProviders(after, "opencode")
	if drift["e2e-oc"] {
		t.Errorf("e2e-oc drift must be clean after push: %v", drift)
	}
	if !drift["e2e-keep"] {
		t.Errorf("e2e-keep (not in central) drift must remain: %v", drift)
	}
}
