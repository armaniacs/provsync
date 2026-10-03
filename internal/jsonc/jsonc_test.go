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

// TestStripJSONCPin pins the exact byte output of StripJSONC so that internal
// refactors cannot change behaviour, not even by one byte.
func TestStripJSONCPin(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trailing comma array", "[1,2,]", "[1,2]"},
		{"trailing comma object", `{"a":1,}`, `{"a":1}`},
		{"slashes in string", `"http://x//y"`, `"http://x//y"`},
		{"escaped quote in string", `"a\"b // c"`, `"a\"b // c"`},
		{"comment before closer", "{\"a\": 1, // c\n}", "{\"a\": 1 \n}"},
		{"comment contains closer", ", // }\nx", ", \nx"},
		{"comment head and trailing comma", "// head\n{\n  \"a\": [1,2,], // c\n}\n", "\n{\n  \"a\": [1,2] \n}\n"},
		{"comment only to eof", "// only comment", ""},
		{"whitespace before closer", "[1, 2, ]", "[1, 2 ]"},
	}
	for _, tt := range tests {
		if got := string(StripJSONC([]byte(tt.input))); got != tt.want {
			t.Errorf("%s: StripJSONC(%q) = %q, want %q", tt.name, tt.input, got, tt.want)
		}
	}
}
