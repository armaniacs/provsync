package jsonc

import (
	"encoding/json"
	"testing"
)

func TestStripJSONC(t *testing.T) {
	in := []byte("{\n" +
		"  // line comment\n" +
		"  \"a\": 1, // trailing comment\n" +
		"  \"b\": [1, 2,],\n" +
		"  \"c\": \"// not a comment\",\n" +
		"  \"d\": \"brace,} inside string\",\n" +
		"}\n")

	var got map[string]any
	if err := json.Unmarshal(StripJSONC(in), &got); err != nil {
		t.Fatalf("StripJSONC produced invalid JSON: %v", err)
	}
	if got["a"].(float64) != 1 {
		t.Errorf("a = %v, want 1", got["a"])
	}
	if got["c"] != "// not a comment" {
		t.Errorf("c = %v, want %q", got["c"], "// not a comment")
	}
	if got["d"] != "brace,} inside string" {
		t.Errorf("d = %v, want %q", got["d"], "brace,} inside string")
	}
	b := got["b"].([]any)
	if len(b) != 2 {
		t.Errorf("b len = %d, want 2", len(b))
	}
}
