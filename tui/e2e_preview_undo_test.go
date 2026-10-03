package main

// プレビューと undo の実連携テスト。
// e2e_contract_test.go のハーネス(buildRealProvsync / isolateHomeEnv /
// writeE2EFile / e2eSelectOnly)を流用し、プレビューが読み取り専用であることと
// 適用→undo の往復でファイルが元に戻ることを実バイナリ相手に固定する。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestE2EPreviewApplyUndoRoundTrip は preview → apply → undo の全路を
// 実バイナリ相手に実行する。プレビューでファイルが変わらないこと、
// 適用で中央どおりになること、undo で適用前の内容に戻ることを検証する。
func TestE2EPreviewApplyUndoRoundTrip(t *testing.T) {
	home := isolateHomeEnv(t)
	bin := buildRealProvsync(t)

	toolPath := filepath.Join(home, ".config", "kilo", "kilo.jsonc")
	writeE2EFile(t, toolPath, `{"provider": {"e2e-rt": {"name": "E2E RT", "npm": "@ai-sdk/x",
"options": {"baseURL": "https://old.example/v1"}, "models": {}}}}`)
	central := filepath.Join(home, ".config", "provsync", "config.json")
	writeE2EFile(t, central, `{"version":1,"providers":{`+
		`"e2e-rt": {"name": "E2E RT", "npm": "@ai-sdk/x",`+
		` "baseURL": "https://central.example/v1", "models": {}}}}`)

	before, err := os.ReadFile(toolPath)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := fetchStatus(bin)
	if err != nil {
		t.Fatalf("fetchStatus: %v", err)
	}
	sel := newSelection(rep)
	e2eSelectOnly(sel, []pair{{Tool: "kilocode", Provider: "e2e-rt"}})

	// プレビュー(--write なし)は読み取り専用で、意味差分を返す。
	if len(sel.previewArgs()) == 0 {
		t.Fatal("previewArgs must produce at least one command (drift not detected?)")
	}
	for _, args := range sel.previewArgs() {
		if len(args) != 4 || args[0] != "push" || args[2] != "--provider" {
			t.Fatalf("contract broken: preview args = %q, want [push <tool> --provider <p>]", args)
		}
		for _, a := range args {
			if a == "--write" {
				t.Fatalf("preview must not use --write: %q", args)
			}
		}
		stdout, stderr, err := runProvsync(bin, args...)
		if err != nil {
			t.Fatalf("preview %q: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout, stderr)
		}
		if !strings.Contains(stdout, "e2e-rt") {
			t.Errorf("preview must show the semantic change lines: %q", stdout)
		}
	}
	afterPreview, err := os.ReadFile(toolPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterPreview) != string(before) {
		t.Error("preview must not modify files")
	}

	// 適用で中央どおりになる。
	for _, args := range sel.applyArgs() {
		stdout, stderr, err := runProvsync(bin, args...)
		if err != nil {
			t.Fatalf("apply %q: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout, stderr)
		}
	}
	got := toolProviderBaseURLs(t, toolPath)
	if got["e2e-rt"] != "https://central.example/v1" {
		t.Errorf("after apply, baseURL = %q, want the central value", got["e2e-rt"])
	}

	// undo(引数なし)で適用前の内容に戻る。
	stdout, stderr, err := runProvsync(bin, "undo")
	if err != nil {
		t.Fatalf("undo: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "restored:") {
		t.Errorf("undo must report the restore: %q", stdout)
	}
	afterUndo, err := os.ReadFile(toolPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterUndo) != string(before) {
		t.Errorf("undo must restore the pre-apply content.\nbefore: %s\nafter: %s", before, afterUndo)
	}
}

// TestE2EMenuPullFlow はメニュー経由の pull(入力→プレビュー→適用)を
// 実バイナリ相手にキー操作で駆動する。メニューが組み立てる引数体系が
// CLI 側とずれた場合の実行時失敗を固定する。
func TestE2EMenuPullFlow(t *testing.T) {
	home := isolateHomeEnv(t)
	bin := buildRealProvsync(t)

	toolPath := filepath.Join(home, ".config", "kilo", "kilo.jsonc")
	writeE2EFile(t, toolPath, `{"provider": {"e2e-menu-new": {"name": "E2E Menu", "npm": "@ai-sdk/x",
"options": {"baseURL": "https://tool.example/v1"}, "models": {}}}}`)
	central := filepath.Join(home, ".config", "provsync", "config.json")
	writeE2EFile(t, central, `{"version":1,"providers":{}}`)

	rep, err := fetchStatus(bin)
	if err != nil {
		t.Fatalf("fetchStatus: %v", err)
	}
	m := newListModel(bin, rep, "en")
	if m.screen != screenList {
		t.Fatalf("drift startup must open the list, got screen %d", m.screen)
	}
	// m でメニューを開き、pull(3 項目目)へ進む。
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m2 := updated.(*listModel)
	for i := 0; i < 2; i++ {
		updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyDown})
		m2 = updated.(*listModel)
	}
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(*listModel)
	if m3.screen != screenMenuPrompt {
		t.Fatalf("pull must open the prompt, got screen %d", m3.screen)
	}
	// ツール名を入力して確定→プレビュー画面。ファイルは変わらない。
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kilocode")})
	m4 := updated.(*listModel)
	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m5 := updated.(*listModel)
	if m5.screen != screenMenuPreview {
		t.Fatalf("pull must show the preview, got screen %d", m5.screen)
	}
	if !strings.Contains(strings.Join(m5.menuPreview, "\n"), "e2e-menu-new") {
		t.Errorf("preview must show the semantic change: %v", m5.menuPreview)
	}
	// y で適用→ツール側の provider がセントラル設定に取り込まれる。
	updated, _ = m5.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m6 := updated.(*listModel)
	if m6.screen != screenMenuResult {
		t.Fatalf("y must run and show the result, got screen %d", m6.screen)
	}
	raw, err := os.ReadFile(central)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "e2e-menu-new") {
		t.Errorf("after menu apply, central must contain the provider:\n%s", raw)
	}
}
