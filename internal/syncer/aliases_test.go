package syncer

import (
	"reflect"
	"testing"

	"github.com/armaniacs/provsync/internal/model"
)

func TestResolveAliases(t *testing.T) {
	aliases := map[string]map[string]string{
		"sonnet": {"kilocode": "claude-sonnet-4-5", "opencode": "anthropic/claude-sonnet-4-5"},
	}
	managed := map[string]model.Provider{
		"llm-01": {
			Models: map[string]any{
				"sonnet": map[string]any{"name": "Sonnet"},
				"gpt-4":  map[string]any{"name": "GPT-4"},
			},
		},
	}

	got, warns := ResolveAliases(managed, aliases, "opencode")

	if len(warns) != 0 {
		t.Errorf("warnings = %v, want none", warns)
	}
	if _, ok := got["llm-01"].Models["anthropic/claude-sonnet-4-5"]; !ok {
		t.Errorf("sonnet must be resolved to the opencode ID: %v", got["llm-01"].Models)
	}
	if _, ok := got["llm-01"].Models["sonnet"]; ok {
		t.Error("the alias key must be replaced")
	}
	if _, ok := got["llm-01"].Models["gpt-4"]; !ok {
		t.Error("non-alias keys must be kept")
	}
}

func TestResolveAliasesDoesNotMutateInput(t *testing.T) {
	aliases := map[string]map[string]string{
		"sonnet": {"opencode": "anthropic/claude-sonnet-4-5"},
	}
	managed := map[string]model.Provider{
		"llm-01": {Models: map[string]any{"sonnet": map[string]any{"name": "Sonnet"}}},
	}
	want := map[string]any{"sonnet": map[string]any{"name": "Sonnet"}}

	ResolveAliases(managed, aliases, "opencode")

	if !reflect.DeepEqual(managed["llm-01"].Models, want) {
		t.Errorf("input models mutated: %v", managed["llm-01"].Models)
	}
}

func TestResolveAliasesUndefinedForToolWarns(t *testing.T) {
	aliases := map[string]map[string]string{
		"sonnet": {"kilocode": "claude-sonnet-4-5"},
	}
	managed := map[string]model.Provider{
		"llm-01": {Models: map[string]any{"sonnet": map[string]any{"name": "Sonnet"}}},
	}

	got, warns := ResolveAliases(managed, aliases, "opencode")

	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want 1", warns)
	}
	if _, ok := got["llm-01"].Models["sonnet"]; !ok {
		t.Error("undefined mapping must pass the key through as-is")
	}
}

func TestResolveAliasesNilAliasesIsNoop(t *testing.T) {
	managed := map[string]model.Provider{
		"llm-01": {Models: map[string]any{"sonnet": map[string]any{"name": "Sonnet"}}},
	}

	got, warns := ResolveAliases(managed, nil, "opencode")

	if len(warns) != 0 {
		t.Errorf("warnings = %v, want none", warns)
	}
	if !reflect.DeepEqual(got, managed) {
		t.Errorf("nil aliases must be a noop copy, got %v", got)
	}
}

func TestResolveAliasesKeepsModelValues(t *testing.T) {
	aliases := map[string]map[string]string{
		"sonnet": {"opencode": "anthropic/claude-sonnet-4-5"},
	}
	managed := map[string]model.Provider{
		"llm-01": {Models: map[string]any{"sonnet": map[string]any{"name": "Sonnet", "id": "x"}}},
	}

	got, _ := ResolveAliases(managed, aliases, "opencode")

	m, ok := got["llm-01"].Models["anthropic/claude-sonnet-4-5"].(map[string]any)
	if !ok {
		t.Fatalf("model value = %#v", got["llm-01"].Models["anthropic/claude-sonnet-4-5"])
	}
	if m["name"] != "Sonnet" || m["id"] != "x" {
		t.Errorf("model value must be copied as-is: %v", m)
	}
}
