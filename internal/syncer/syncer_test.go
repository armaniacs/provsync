package syncer

import (
	"testing"

	"github.com/armaniacs/provsync/internal/model"
)

func TestMergeToolProvidersOverwritesAndKeepsOthers(t *testing.T) {
	base := map[string]model.Provider{
		"keep":   {Name: "keep"},
		"shared": {Name: "old"},
	}
	incoming := map[string]model.Provider{
		"shared": {Name: "new"},
		"added":  {Name: "added"},
	}
	got := MergeToolProviders(base, incoming, "kilocode")
	if got["keep"].Name != "keep" {
		t.Error("base-only provider lost")
	}
	if got["added"].Name != "added" {
		t.Error("incoming provider missing")
	}
	if got["shared"].Name != "new" {
		t.Error("incoming should overwrite")
	}
	if base["shared"].Name != "old" {
		t.Error("MergeToolProviders must not mutate base")
	}
}

func TestMergeToolProvidersPreservesOtherNamespaces(t *testing.T) {
	base := map[string]model.Provider{
		"p": {
			Name: "old",
			Extras: map[string]map[string]any{
				"kilocode": {"reasoning": true},
				"opencode": {"opencodeOnly": 1},
			},
		},
	}
	incoming := map[string]model.Provider{
		"p": {
			Name: "new",
			Extras: map[string]map[string]any{
				"opencode": {"fresh": 2},
			},
		},
	}
	got := MergeToolProviders(base, incoming, "opencode")
	if got["p"].Name != "new" {
		t.Errorf("name = %q, want new", got["p"].Name)
	}
	extras := got["p"].Extras
	if extras["kilocode"] == nil || extras["kilocode"]["reasoning"] != true {
		t.Errorf("kilocode namespace must be preserved: %v", extras)
	}
	if extras["opencode"]["fresh"] != 2 {
		t.Errorf("opencode namespace must be replaced: %v", extras["opencode"])
	}
	if _, stale := extras["opencode"]["opencodeOnly"]; stale {
		t.Errorf("opencode namespace must not keep stale keys: %v", extras["opencode"])
	}
}

func TestMergeToolProvidersIncomingWithoutExtrasClearsNamespace(t *testing.T) {
	base := map[string]model.Provider{
		"p": {Extras: map[string]map[string]any{"kilocode": {"reasoning": true}}},
	}
	incoming := map[string]model.Provider{
		"p": {Name: "p"}, // kilocode 側で extras が削除された
	}
	got := MergeToolProviders(base, incoming, "kilocode")
	if got["p"].Extras != nil {
		t.Errorf("namespace should be cleared: %v", got["p"].Extras)
	}
}

func TestFilterProvidersEmptyReturnsAll(t *testing.T) {
	all := map[string]model.Provider{"a": {}, "b": {}}
	got, err := FilterProviders(all, nil)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestFilterProvidersSelects(t *testing.T) {
	all := map[string]model.Provider{"a": {}, "b": {}, "c": {}}
	got, err := FilterProviders(all, []string{"a", "c"})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	if _, ok := got["b"]; ok {
		t.Error("b should be filtered out")
	}
}

func TestFilterProvidersMissingErrors(t *testing.T) {
	all := map[string]model.Provider{"a": {}}
	if _, err := FilterProviders(all, []string{"zzz"}); err == nil {
		t.Error("expected error for missing provider")
	}
}
