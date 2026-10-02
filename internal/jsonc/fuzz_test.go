package jsonc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzStripJSONC(f *testing.F) {
	seeds := []string{
		`{"a": "http://x/y"}`,
		`{"a": [1, 2,],}`,
		"{\n  // c\n  \"a\": 1, // t\n}",
		`{"a": "\\"}`,
		`{"a": "x, }"}`,
		`{"a": "a//b", "b": "c//d"}`,
		`{"a": ""}`,
		`{}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		out := StripJSONC(in)
		if !json.Valid(in) {
			return
		}
		var a, b any
		// 数値は json.Number で比較する。巨大な数値は float64 で表現できないが、
		// StripJSONC の正しさの判定には数値の文字列一致で十分である。
		da := json.NewDecoder(bytes.NewReader(in))
		da.UseNumber()
		if err := da.Decode(&a); err != nil {
			t.Fatal(err)
		}
		db := json.NewDecoder(bytes.NewReader(out))
		db.UseNumber()
		if err := db.Decode(&b); err != nil {
			t.Fatalf("valid JSON became invalid: %q -> %q", in, out)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("meaning changed: %q -> %q", in, out)
		}
	})
}
