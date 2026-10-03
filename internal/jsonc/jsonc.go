package jsonc

// StripJSONC removes // line comments and trailing commas from a JSONC
// document in a single pass so it can be parsed by encoding/json. Comment
// markers and commas inside string literals are preserved.
func StripJSONC(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inString := false
	escaped := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if end := commentEnd(b, i); end >= 0 {
			// Only line comments end at a newline; the newline itself is kept.
			if end < len(b) && b[end] == '\n' {
				out = append(out, b[end])
			}
			i = end
			continue
		}
		if c == ',' && isTrailingComma(b, i) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// commentEnd returns the index just past the comment starting at b[i], or -1
// when b[i] does not start one. The terminator (the newline of a // comment)
// is not consumed, so callers can keep it.
func commentEnd(b []byte, i int) int {
	if i+1 >= len(b) || b[i] != '/' || b[i+1] != '/' {
		return -1
	}
	j := i + 2
	for j < len(b) && b[j] != '\n' {
		j++
	}
	return j
}

// isTrailingComma reports whether the comma at b[i] is trailing: only
// whitespace and comments may separate it from the next '}' or ']'.
// Comments must be skipped here: one can sit between the comma and the
// closer, and a '}' inside a comment must not count as the closer.
func isTrailingComma(b []byte, i int) bool {
	for j := i + 1; j < len(b); {
		c := b[j]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			j++
			continue
		}
		if end := commentEnd(b, j); end >= 0 {
			j = end
			continue
		}
		return c == '}' || c == ']'
	}
	return false
}
