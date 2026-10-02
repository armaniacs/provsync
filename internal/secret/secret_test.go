package secret

import "testing"

func TestIsKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"apiKey", true},
		{"APIKEY", true},
		{"api_key", true},
		{"token", true},
		{"secret", true},
		{"password", true},
		{"accessToken", true},
		{"access_token", true},
		{"name", false},
		{"baseURL", false},
		{"models", false},
	}
	for _, tt := range tests {
		if got := IsKey(tt.key); got != tt.want {
			t.Errorf("IsKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestMaskLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "masks apiKey value",
			in:   `  "apiKey": "sk-abc",`,
			want: `  "apiKey": "********",`,
		},
		{
			name: "masks token value",
			in:   `  "token": "t1"`,
			want: `  "token": "********"`,
		},
		{
			name: "keeps non-secret values",
			in:   `  "name": "x"`,
			want: `  "name": "x"`,
		},
		{
			name: "masks nested secret",
			in:   `  "options": {"token": "t1"}`,
			want: `  "options": {"token": "********"}`,
		},
		{
			name: "masks escaped value as one value",
			in:   `  "apiKey": "a\"b"`,
			want: `  "apiKey": "********"`,
		},
		{
			name: "keeps comment-like strings",
			in:   `  "url": "http://x/y"`,
			want: `  "url": "http://x/y"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskLines(tt.in); got != tt.want {
				t.Errorf("MaskLines(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
