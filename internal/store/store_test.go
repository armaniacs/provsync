package store

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/model"
)

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := fsutil.WriteFileAtomic(path, data); err != nil {
		t.Fatal(err)
	}
}

func TestMarshalLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &model.Config{
		Version: model.Version,
		Providers: map[string]model.Provider{
			"llm-01": {Name: "LLM 01", NPM: "pkg", BaseURL: "https://a/v1"},
		},
	}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	write(t, path, data)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Providers) != 1 || got.Providers["llm-01"].BaseURL != "https://a/v1" {
		t.Errorf("unexpected providers: %+v", got.Providers)
	}
	if got.Version != model.Version {
		t.Errorf("version = %d, want %d", got.Version, model.Version)
	}
}

func TestMarshalSortsTopLevelKeys(t *testing.T) {
	data, err := Marshal(model.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Index(text, `"providers"`) > strings.Index(text, `"version"`) {
		t.Errorf("keys not sorted:\n%s", text)
	}
}

func TestMarshalPreservesExplicitVersion(t *testing.T) {
	data, err := Marshal(&model.Config{Version: 2, Providers: map[string]model.Provider{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"version": 2`) {
		t.Errorf("explicit version not preserved:\n%s", data)
	}
}

func TestLoadMissingErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadResetsZeroVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write(t, path, []byte(`{"version": 0, "providers": {}}`))
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != model.Version {
		t.Errorf("version = %d, want %d", got.Version, model.Version)
	}
	if got.Providers == nil {
		t.Error("providers should be initialized")
	}
}
