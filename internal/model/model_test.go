package model

import (
	"encoding/json"
	"testing"
)

func TestProviderExtrasRoundTrip(t *testing.T) {
	p := Provider{
		Name:   "llm-01",
		NPM:    "@ai-sdk/openai-compatible",
		Models: map[string]any{"m": map[string]any{"name": "m"}},
	}
	p.SetExtra("kilocode", map[string]any{"reasoning": true})

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Provider
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	extra := got.Extra("kilocode")
	if extra == nil {
		t.Fatal("kilocode extras lost")
	}
	if extra["reasoning"] != true {
		t.Errorf("reasoning = %v, want true", extra["reasoning"])
	}
	if got.Name != "llm-01" {
		t.Errorf("Name = %q", got.Name)
	}
}

func TestSetExtraEmptyIsNoop(t *testing.T) {
	var p Provider
	p.SetExtra("opencode", nil)
	p.SetExtra("opencode", map[string]any{})
	if p.Extras != nil {
		t.Errorf("Extras should stay nil, got %v", p.Extras)
	}
	if p.Extra("opencode") != nil {
		t.Error("Extra should be nil")
	}
}
