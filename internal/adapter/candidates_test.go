package adapter

// 設定ファイル候補の解決テスト。オフィシャルの読み順に準拠する:
//   - kilocode: kilo.jsonc → kilo.json → config.json
//   - opencode: opencode.jsonc → opencode.json → config.json

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func candidateRoot(t *testing.T) Root {
	t.Helper()
	dir := t.TempDir()
	return Root{ConfigHome: dir, StateHome: filepath.Join(dir, "state")}
}

// TestResolveKilocodeJson は kilo.json だけがあるときに解決されることを検証する。
func TestResolveKilocodeJson(t *testing.T) {
	root := candidateRoot(t)
	writeFile(t, filepath.Join(root.ConfigHome, "kilo", "kilo.json"),
		`{"provider": {"k1": {"name": "K1"}}}`)
	a, err := Get("kilocode", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(a.Path(), filepath.Join("kilo", "kilo.json")) {
		t.Errorf("Path = %q, want kilo.json", a.Path())
	}
	got, _, err := a.Pull()
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, ok := got["k1"]; !ok {
		t.Errorf("Pull must read kilo.json: %v", got)
	}
}

// TestResolvePrefersJsonc は両方あるときに .jsonc が優先されることを検証する
// (opencode オフィシャルと同じ優先順)。
func TestResolvePrefersJsonc(t *testing.T) {
	root := candidateRoot(t)
	writeFile(t, filepath.Join(root.ConfigHome, "opencode", "opencode.json"),
		`{"provider": {"from-json": {"name": "J"}}}`)
	writeFile(t, filepath.Join(root.ConfigHome, "opencode", "opencode.jsonc"),
		`{"provider": {"from-jsonc": {"name": "JC"}}}`)
	a, err := Get("opencode", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(a.Path(), "opencode.jsonc") {
		t.Errorf("Path = %q, want opencode.jsonc", a.Path())
	}
	got, _, err := a.Pull()
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, ok := got["from-jsonc"]; !ok {
		t.Errorf("Pull must read opencode.jsonc: %v", got)
	}
	if _, ok := got["from-json"]; ok {
		t.Errorf("Pull must not merge opencode.json: %v", got)
	}
}

// TestResolveFallsBackToConfigJson は config.json だけのときに解決されることを検証する。
func TestResolveFallsBackToConfigJson(t *testing.T) {
	root := candidateRoot(t)
	writeFile(t, filepath.Join(root.ConfigHome, "opencode", "config.json"),
		`{"provider": {"c1": {"name": "C1"}}}`)
	a, err := Get("opencode", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(a.Path(), "config.json") {
		t.Errorf("Path = %q, want config.json", a.Path())
	}
}

// TestResolveMissingReturnsPrimary は存在しないときに先頭候補を返すことを検証する
// (作成時の置き場所とエラー表示用)。
func TestResolveMissingReturnsPrimary(t *testing.T) {
	root := candidateRoot(t)
	a, err := Get("kilocode", root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root.ConfigHome, "kilo", "kilo.jsonc")
	if a.Path() != want {
		t.Errorf("Path = %q, want %q", a.Path(), want)
	}
	b, err := Get("opencode", root)
	if err != nil {
		t.Fatal(err)
	}
	wantOpencode := filepath.Join(root.ConfigHome, "opencode", "opencode.jsonc")
	if b.Path() != wantOpencode {
		t.Errorf("Path = %q, want %q", b.Path(), wantOpencode)
	}
}

// TestOpencodeJsoncWithComments は opencode.jsonc のコメントと末尾カンマが
// 解釈されることを検証する(オフィシャルは両拡張子を JSONC として読む)。
func TestOpencodeJsoncWithComments(t *testing.T) {
	root := candidateRoot(t)
	writeFile(t, filepath.Join(root.ConfigHome, "opencode", "opencode.jsonc"),
		`{
  // comment
  "provider": {
    "c1": {"name": "C1",},
  },
}`)
	a, err := Get("opencode", root)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := a.Pull()
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, ok := got["c1"]; !ok {
		t.Errorf("Pull must parse JSONC: %v", got)
	}
}

// TestResolveBrokenSymlinkIsNotSkipped は壊れリンクが黙って読み飛ばされず、
// 候補として採用されることを検証する(後段が正確なリンクエラーを出すため)。
func TestResolveBrokenSymlinkIsNotSkipped(t *testing.T) {
	root := candidateRoot(t)
	broken := filepath.Join(root.ConfigHome, "kilo", "kilo.jsonc")
	if err := os.MkdirAll(filepath.Dir(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing.jsonc"), broken); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root.ConfigHome, "kilo", "kilo.json"),
		`{"provider": {"k1": {"name": "K1"}}}`)
	a, err := Get("kilocode", root)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path() != broken {
		t.Errorf("Path = %q, want the broken symlink %q", a.Path(), broken)
	}
	if _, _, err := a.Pull(); err == nil {
		t.Error("Pull of a broken symlink must fail")
	}
}
