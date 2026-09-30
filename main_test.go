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
		"    \"vs\": { \"npm\": \"@ai-sdk/openai-compatible\", \"models\": { \"a\": { \"name\": \"a\" } } },\n" +
		"    \"sakura\": { \"models\": { \"s\": { \"name\": \"s\" } } },\n" +
		"    \"vs_inoue\": { \"models\": { \"i\": { \"name\": \"i\" } } },\n" +
		"  },\n" +
		"}\n"
	if err := os.WriteFile(source, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	origTarget = []byte("{\"provider\":{\"sakura\":{\"models\":{\"stale\":{}}},\"vsakura\":{\"models\":{}}}}")
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
	if _, ok := providers["vs"]; !ok {
		t.Error("vs missing")
	}
	if _, ok := providers["vs_inoue"]; !ok {
		t.Error("vs_inoue missing")
	}
	if _, ok := providers["vsakura"]; !ok {
		t.Error("vsakura should be preserved")
	}
	sakura := providers["sakura"].(map[string]any)
	models := sakura["models"].(map[string]any)
	if _, ok := models["stale"]; ok {
		t.Error("sakura should be overwritten")
	}
	if _, ok := models["s"]; !ok {
		t.Error("sakura model s missing")
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
