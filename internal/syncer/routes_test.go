package syncer

import (
	"testing"

	"github.com/armaniacs/provsync/internal/model"
)

func TestSelectRoutePrefersFirstConfigured(t *testing.T) {
	providers := map[string]model.Provider{
		"direct":     {APIKeyEnv: "DIRECT_API_KEY"},
		"openrouter": {APIKeyEnv: "OPENROUTER_API_KEY"},
	}
	lookup := func(name string) bool { return name == "OPENROUTER_API_KEY" }

	key, ok := SelectRoute([]string{"direct", "openrouter"}, providers, lookup)
	if !ok || key != "openrouter" {
		t.Errorf("SelectRoute = (%q, %v), want (openrouter, true)", key, ok)
	}
}

func TestSelectRouteFirstWhenConfigured(t *testing.T) {
	providers := map[string]model.Provider{
		"direct":     {APIKeyEnv: "DIRECT_API_KEY"},
		"openrouter": {APIKeyEnv: "OPENROUTER_API_KEY"},
	}
	lookup := func(name string) bool { return true }

	key, ok := SelectRoute([]string{"direct", "openrouter"}, providers, lookup)
	if !ok || key != "direct" {
		t.Errorf("SelectRoute = (%q, %v), want (direct, true)", key, ok)
	}
}

func TestSelectRouteNoneConfigured(t *testing.T) {
	providers := map[string]model.Provider{
		"direct":     {APIKeyEnv: "DIRECT_API_KEY"},
		"openrouter": {APIKeyEnv: "OPENROUTER_API_KEY"},
	}
	lookup := func(string) bool { return false }

	if _, ok := SelectRoute([]string{"direct", "openrouter"}, providers, lookup); ok {
		t.Error("SelectRoute must return ok=false when no candidate is configured")
	}
}

func TestSelectRouteEmptyAPIKeyNeedsNoEnv(t *testing.T) {
	providers := map[string]model.Provider{
		"free": {APIKeyEnv: ""},
	}
	lookup := func(string) bool { return false }

	key, ok := SelectRoute([]string{"free"}, providers, lookup)
	if !ok || key != "free" {
		t.Errorf("SelectRoute = (%q, %v), want (free, true)", key, ok)
	}
}

func TestSelectRouteSkipsUnknownCandidates(t *testing.T) {
	providers := map[string]model.Provider{
		"openrouter": {APIKeyEnv: "OPENROUTER_API_KEY"},
	}
	lookup := func(string) bool { return true }

	key, ok := SelectRoute([]string{"missing", "openrouter"}, providers, lookup)
	if !ok || key != "openrouter" {
		t.Errorf("SelectRoute = (%q, %v), want (openrouter, true)", key, ok)
	}
}
