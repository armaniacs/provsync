package syncer

import "testing"

func TestMergeOverwritesAndKeepsOthers(t *testing.T) {
	source := map[string]any{
		"provider": map[string]any{
			"vs": map[string]any{
				"npm":    "@ai-sdk/openai-compatible",
				"models": map[string]any{"new": map[string]any{"name": "new"}},
			},
			"sakura": map[string]any{
				"models": map[string]any{"s": map[string]any{"name": "s"}},
			},
			"vs_inoue": map[string]any{
				"models": map[string]any{"i": map[string]any{"name": "i"}},
			},
		},
	}
	target := map[string]any{
		"provider": map[string]any{
			"vsakura": map[string]any{"models": map[string]any{}},
			"sakura":  map[string]any{"models": map[string]any{"stale": map[string]any{}}},
		},
	}

	got, err := Merge(source, target, []string{"vs_inoue", "sakura", "vs"})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	providers := got["provider"].(map[string]any)

	if _, ok := providers["vsakura"]; !ok {
		t.Error("vsakura should be preserved")
	}
	if _, ok := providers["vs_inoue"]; !ok {
		t.Error("vs_inoue should be added")
	}
	vs := providers["vs"].(map[string]any)
	if vs["npm"] != "@ai-sdk/openai-compatible" {
		t.Errorf("vs.npm = %v", vs["npm"])
	}
	sakura := providers["sakura"].(map[string]any)
	models := sakura["models"].(map[string]any)
	if _, ok := models["stale"]; ok {
		t.Error("sakura should be overwritten, stale model still present")
	}
	if _, ok := models["s"]; !ok {
		t.Error("sakura model s missing")
	}
}

func TestMergeMissingProviderErrors(t *testing.T) {
	source := map[string]any{"provider": map[string]any{}}
	target := map[string]any{"provider": map[string]any{}}
	if _, err := Merge(source, target, []string{"vs"}); err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestMergeCreatesProviderSectionWhenAbsent(t *testing.T) {
	source := map[string]any{
		"provider": map[string]any{
			"vs": map[string]any{"models": map[string]any{}},
		},
	}
	got, err := Merge(source, map[string]any{}, []string{"vs"})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	providers, ok := got["provider"].(map[string]any)
	if !ok {
		t.Fatal("provider section not created")
	}
	if _, ok := providers["vs"]; !ok {
		t.Error("vs not added")
	}
}
