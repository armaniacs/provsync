// Package secret は秘密情報らしいキーの判定と、表示用のマスクを提供する。
package secret

import (
	"regexp"
	"strings"
)

// IsKey は秘密情報らしいキー名かを判定する(大文字小文字は区別しない)。
func IsKey(key string) bool {
	switch strings.ToLower(key) {
	case "apikey", "api_key", "token", "secret", "password", "accesstoken", "access_token":
		return true
	}
	return false
}

var jsonStringField = regexp.MustCompile(`("([^"\\]|\\.)*")(\s*:\s*)("([^"\\]|\\.)*")`)

// MaskLines は JSON 風の行 "key": "value" のうち key が秘密らしいものの value を伏せる。
func MaskLines(text string) string {
	return jsonStringField.ReplaceAllStringFunc(text, func(m string) string {
		sub := jsonStringField.FindStringSubmatch(m)
		key := strings.Trim(sub[1], `"`)
		if !IsKey(key) {
			return m
		}
		return sub[1] + sub[3] + `"********"`
	})
}
