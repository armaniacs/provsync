package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/armaniacs/provsync/internal/model"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewRoot(t *testing.T) {
	home := t.TempDir()

	t.Run("defaults when unset", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("XDG_STATE_HOME", "")
		r := NewRoot(home)
		if r.ConfigHome != filepath.Join(home, ".config") {
			t.Errorf("ConfigHome = %q, want %q", r.ConfigHome, filepath.Join(home, ".config"))
		}
		if r.StateHome != filepath.Join(home, ".local", "state") {
			t.Errorf("StateHome = %q, want %q", r.StateHome, filepath.Join(home, ".local", "state"))
		}
	})

	t.Run("respects XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/custom")
		t.Setenv("XDG_STATE_HOME", "")
		r := NewRoot(home)
		if got := r.CentralConfigPath(); got != filepath.Join("/custom", "provsync", "config.json") {
			t.Errorf("CentralConfigPath() = %q, want /custom/provsync/config.json", got)
		}
	})

	t.Run("respects XDG_STATE_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("XDG_STATE_HOME", "/s")
		r := NewRoot(home)
		if got := r.StateDir(); got != filepath.Join("/s", "provsync") {
			t.Errorf("StateDir() = %q, want /s/provsync", got)
		}
	})
}

func TestKilocodePullDecodesExtrasAndWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kilo.jsonc")
	writeFile(t, path, `{
  // line comment
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1", "timeout": 30, "apiKey": "SECRET" },
      "models": { "m": { "name": "m" } },
      "reasoning": true
    }
  }
}`)

	a := &kilocode{path: path}
	providers, warnings, err := a.Pull()
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	p, ok := providers["llm-01"]
	if !ok {
		t.Fatal("llm-01 missing")
	}
	if p.Name != "LLM 01" || p.NPM != "@ai-sdk/openai-compatible" || p.BaseURL != "https://a.example/v1" {
		t.Errorf("decoded fields wrong: %+v", p)
	}
	extra := p.Extra("kilocode")
	if extra["reasoning"] != true {
		t.Errorf("reasoning extra = %v", extra["reasoning"])
	}
	opts, _ := extra["options"].(map[string]any)
	if opts["timeout"] != float64(30) {
		t.Errorf("options.timeout = %v", opts["timeout"])
	}
	if _, leaked := opts["apiKey"]; leaked {
		t.Error("plaintext apiKey must not be stored in extras")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "options.apiKey") {
		t.Errorf("warnings = %v, want one mentioning options.apiKey", warnings)
	}
}

func TestPullWarnsAndDropsEntryLevelSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kilo.jsonc")
	writeFile(t, path, `{
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "token": "SECRET",
      "npm": "pkg",
      "options": { "baseURL": "https://a.example/v1" }
    }
  }
}`)
	a := &kilocode{path: path}
	providers, warnings, err := a.Pull()
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	p := providers["llm-01"]
	if _, leaked := p.Extra("kilocode")["token"]; leaked {
		t.Error("entry-level secret must not be stored in extras")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `"token"`) {
		t.Errorf("warnings = %v, want one mentioning token", warnings)
	}
}

func TestPushPreservesExistingSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kilo.jsonc")
	writeFile(t, path, `{
  "provider": {
    "llm-01": {
      "name": "old",
      "npm": "old",
      "options": { "baseURL": "https://old/v1", "apiKey": "SECRET" },
      "token": "ENTRY_SECRET"
    }
  }
}`)
	managed := map[string]model.Provider{
		"llm-01": {Name: "LLM 01", NPM: "pkg", BaseURL: "https://new/v1"},
	}
	a := &kilocode{path: path}
	out, err := a.Push(managed)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	entry := doc["provider"].(map[string]any)["llm-01"].(map[string]any)
	if entry["name"] != "LLM 01" {
		t.Errorf("name = %v, want updated", entry["name"])
	}
	opts := entry["options"].(map[string]any)
	if opts["apiKey"] != "SECRET" {
		t.Errorf("options.apiKey must survive push, got %v", opts["apiKey"])
	}
	if entry["token"] != "ENTRY_SECRET" {
		t.Errorf("entry-level secret must survive push, got %v", entry["token"])
	}
}

func TestProjectDropsToolInvisibleFields(t *testing.T) {
	managed := map[string]model.Provider{
		"llm-01": {
			Name:      "LLM 01",
			APIKeyEnv: "LLM01_API_KEY",
			Extras: map[string]map[string]any{
				"kilocode": {"reasoning": true},
			},
		},
	}
	ko := projectProviders("kilocode", managed, true)
	if ko["llm-01"].APIKeyEnv != "LLM01_API_KEY" {
		t.Error("kilocode projection must keep apiKeyEnv")
	}
	if _, ok := ko["llm-01"].Extras["opencode"]; ok {
		t.Error("projection must not invent namespaces")
	}

	oc := projectProviders("opencode", managed, false)
	if oc["llm-01"].APIKeyEnv != "" {
		t.Error("opencode projection must drop apiKeyEnv")
	}
	if _, ok := oc["llm-01"].Extras["kilocode"]; ok {
		t.Error("opencode projection must drop other tools' namespaces")
	}
}

func TestKilocodePushPreservesUnknownAndUnmanaged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kilo.jsonc")
	writeFile(t, path, `{
  "$schema": "schema.json",
  "provider": {
    "legacy": { "name": "legacy", "npm": "x", "options": { "baseURL": "https://legacy/v1" }, "models": {} }
  }
}`)

	managed := map[string]model.Provider{
		"llm-01": {
			Name:    "LLM 01",
			NPM:     "@ai-sdk/openai-compatible",
			BaseURL: "https://a.example/v1",
			Models:  map[string]any{"m": map[string]any{"name": "m"}},
			Extras: map[string]map[string]any{
				"kilocode": {"reasoning": true, "options": map[string]any{"timeout": 30}},
			},
		},
	}

	a := &kilocode{path: path}
	out, err := a.Push(managed)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if doc["$schema"] != "schema.json" {
		t.Error("non-provider top-level key lost")
	}
	section := doc["provider"].(map[string]any)
	if _, ok := section["legacy"]; !ok {
		t.Error("unmanaged provider lost")
	}
	entry := section["llm-01"].(map[string]any)
	if entry["reasoning"] != true {
		t.Errorf("extras reasoning lost: %v", entry)
	}
	opts := entry["options"].(map[string]any)
	if opts["baseURL"] != "https://a.example/v1" {
		t.Errorf("baseURL = %v", opts["baseURL"])
	}
	if opts["timeout"] != float64(30) {
		t.Errorf("extras options.timeout lost: %v", opts)
	}
}

func TestOpencodePushIgnoresAPIKeyEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	writeFile(t, path, `{"provider": {}}`)

	managed := map[string]model.Provider{
		"llm-01": {Name: "LLM 01", NPM: "pkg", BaseURL: "https://a/v1", APIKeyEnv: "LLM01_API_KEY"},
	}
	a := &opencode{path: path}
	out, err := a.Push(managed)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	entry := doc["provider"].(map[string]any)["llm-01"].(map[string]any)
	opts := entry["options"].(map[string]any)
	if _, ok := opts["apiKeyEnv"]; ok {
		t.Error("opencode must not render apiKeyEnv")
	}
}

func TestGetNamesAndAlias(t *testing.T) {
	root := Root{ConfigHome: "/tmp/cfg", StateHome: "/tmp/state"}
	a, err := Get("kilo", root)
	if err != nil {
		t.Fatalf("alias: %v", err)
	}
	if a.Name() != "kilocode" {
		t.Errorf("alias Name = %q", a.Name())
	}
	if _, err := Get("nope", root); err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestPullMissingFileErrors(t *testing.T) {
	a := &kilocode{path: filepath.Join(t.TempDir(), "nope.jsonc")}
	if _, _, err := a.Pull(); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestOpencodePullDecodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	writeFile(t, path, `{
  "$schema": "opencode.schema.json",
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1" },
      "models": { "m": { "name": "m" } }
    }
  }
}`)
	a := &opencode{path: path}
	providers, warnings, err := a.Pull()
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v", warnings)
	}
	p := providers["llm-01"]
	if p.Name != "LLM 01" || p.BaseURL != "https://a.example/v1" {
		t.Errorf("decoded = %+v", p)
	}
	if p.Models == nil || p.Models["m"] == nil {
		t.Errorf("models not decoded: %+v", p.Models)
	}
}
