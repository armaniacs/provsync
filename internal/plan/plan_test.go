package plan

import (
	"testing"

	"github.com/armaniacs/provsync/internal/model"
)

func TestProvidersDiff(t *testing.T) {
	desired := map[string]model.Provider{
		"added":   {Name: "added"},
		"updated": {Name: "new", NPM: "pkg"},
		"same":    {Name: "same"},
	}
	current := map[string]model.Provider{
		"updated": {Name: "old", NPM: "pkg"},
		"same":    {Name: "same"},
		"ignored": {Name: "ignored"},
	}
	got := ProvidersDiff(desired, current)

	byKey := map[string]ProviderChange{}
	for _, c := range got {
		byKey[c.Key] = c
	}
	if byKey["added"].Op != "added" {
		t.Errorf("added op = %q", byKey["added"].Op)
	}
	if byKey["updated"].Op != "updated" {
		t.Errorf("updated op = %q", byKey["updated"].Op)
	}
	if len(byKey["updated"].Fields) != 1 || byKey["updated"].Fields[0] != "name" {
		t.Errorf("updated fields = %v", byKey["updated"].Fields)
	}
	if byKey["same"].Op != "unchanged" {
		t.Errorf("same op = %q", byKey["same"].Op)
	}
	if _, ok := byKey["ignored"]; ok {
		t.Error("current-only key should not appear in managed diff")
	}
}

func TestProvidersDiffModels(t *testing.T) {
	desired := map[string]model.Provider{
		"p": {Models: map[string]any{"a": 1}},
	}
	current := map[string]model.Provider{
		"p": {Models: map[string]any{"a": 2}},
	}
	got := ProvidersDiff(desired, current)
	if got[0].Op != "updated" || got[0].Fields[0] != "models" {
		t.Errorf("got %+v", got[0])
	}
}

func TestPlanChanged(t *testing.T) {
	same := Plan{Changes: []FileChange{{Before: []byte("a"), After: []byte("a")}}}
	if same.Changed() {
		t.Error("identical bytes should not be changed")
	}
	diff := Plan{Changes: []FileChange{{Before: []byte("a"), After: []byte("b")}}}
	if !diff.Changed() {
		t.Error("different bytes should be changed")
	}
}

func TestChangedFieldsDetectsExtras(t *testing.T) {
	a := model.Provider{Name: "n", Extras: map[string]map[string]any{"kilocode": {"x": 1}}}
	b := model.Provider{Name: "n", Extras: map[string]map[string]any{"opencode": {"y": 2}}}
	if got := ChangedFields(a, b); len(got) == 0 {
		t.Error("ChangedFields should detect extras difference")
	}
}

func TestChangedFieldsTreatsNilAndEmptyModelsAsEqual(t *testing.T) {
	a := model.Provider{Name: "n", Models: nil}
	b := model.Provider{Name: "n", Models: map[string]any{}}
	if got := ChangedFields(a, b); len(got) != 0 {
		t.Errorf("nil and empty models should be equal, got %v", got)
	}
	c := model.Provider{Name: "n", Models: map[string]any{"m": 1}}
	if got := ChangedFields(a, c); len(got) != 1 || got[0] != "models" {
		t.Errorf("non-empty models should differ, got %v", got)
	}
}
