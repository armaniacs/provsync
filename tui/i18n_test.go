package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/armaniacs/provsync/internal/i18n"
)

var cjkRe = regexp.MustCompile(`[\p{Hiragana}\p{Katakana}\p{Han}]`)

// TestViewLocalizedInEn は en ロケールの View が英語文言になり、
// 日本語文字を含まないことを検証する(BDD「英語ロケールで tui を起動すると英語で表示される」の単体相当)。
func TestViewLocalizedInEn(t *testing.T) {
	jaView := newListModel("provsync-unused", testReport(), "ja").View()
	if !strings.Contains(jaView, "セントラル設定:") {
		t.Errorf("ja view must use the ja label:\n%s", jaView)
	}

	enView := newListModel("provsync-unused", testReport(), "en").View()
	if !strings.Contains(enView, "central config:") {
		t.Errorf("en view must use the en label:\n%s", enView)
	}
	if !strings.Contains(enView, "↑↓/jk move · space select · enter confirm · m menu · q quit · ? help") {
		t.Errorf("en view must show the footer guide:\n%s", enView)
	}
	if cjkRe.MatchString(enView) {
		t.Errorf("en view must not contain CJK:\n%s", enView)
	}
}

// TestNoDriftStartsAtMenuLocalizedInEn は差分なし起動でコマンドメニューに
// 直接入ることと、その en 描画を検証する(行き止まりの一覧画面は廃止)。
func TestNoDriftStartsAtMenuLocalizedInEn(t *testing.T) {
	r := testReport()
	for i := range r.Tools {
		r.Tools[i].Drift = nil
		r.Tools[i].DriftEntries = nil
	}
	m := newListModel("provsync-unused", r, "en")
	if m.screen != screenMenu {
		t.Fatalf("no-drift startup must open the menu, got screen %d", m.screen)
	}
	enView := m.View()
	if !strings.Contains(enView, "commands") {
		t.Errorf("en menu view must show the menu title:\n%s", enView)
	}
	if cjkRe.MatchString(enView) {
		t.Errorf("en menu view must not contain CJK:\n%s", enView)
	}
}

// TestFetchStatusParsesFakeBinOutput はフェイクバイナリ(status --json の JSON を出す
// スクリプト)で fetchStatus のパースを検証する。
func TestFetchStatusParsesFakeBinOutput(t *testing.T) {
	bin := fakeBin(t, `{"schemaVersion":1,"central":{"path":"/c","exists":true,"providers":2},"tools":[]}`)
	rep, err := fetchStatus(bin)
	if err != nil {
		t.Fatalf("fetchStatus: %v", err)
	}
	if rep.SchemaVersion != 1 || !rep.Central.Exists || rep.Central.Providers != 2 {
		t.Errorf("report = %+v", rep)
	}
}

// TestFetchStatusErrorIsNeutralMessage は fetchStatus のエラーが言語中立の
// Message で、en 描画が Localize 経由で日本語文字を含まないことを検証する。
func TestFetchStatusErrorIsNeutralMessage(t *testing.T) {
	bin := fakeBin(t, `{ not json`)
	_, err := fetchStatus(bin)
	if err == nil {
		t.Fatal("invalid JSON must error")
	}
	var m *i18n.Message
	if !errors.As(err, &m) {
		t.Fatalf("error must be an *i18n.Message, got %T: %v", err, err)
	}
	got := i18n.Localize("en", err)
	if !strings.Contains(got, "invalid output from provsync status --json") {
		t.Errorf("en error must use the en message: %q", got)
	}
	if cjkRe.MatchString(got) {
		t.Errorf("en error must not contain CJK: %q", got)
	}
	if ja := i18n.Localize("ja", err); !strings.Contains(ja, "出力が不正です") {
		t.Errorf("ja error must use the ja message: %q", ja)
	}
}

// TestFetchStatusRejectsUnknownSchemaVersion は未対応 schemaVersion のエラーを検証する。
func TestFetchStatusRejectsUnknownSchemaVersion(t *testing.T) {
	bin := fakeBin(t, `{"schemaVersion":99}`)
	_, err := fetchStatus(bin)
	if err == nil {
		t.Fatal("unknown schemaVersion must error")
	}
	var m *i18n.Message
	if !errors.As(err, &m) || m.ID != "err.tui.schema" {
		t.Fatalf("error must be err.tui.schema, got %v", err)
	}
}

// fakeBin は引数に応じて content を標準出力へ流す実行可能スクリプトを作る。
func fakeBin(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-provsync")
	script := "#!/bin/sh\nprintf '%s'\n"
	if err := os.WriteFile(bin, []byte(strings.ReplaceAll(script, "%s", strings.ReplaceAll(content, "'", "'\\''"))), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}
