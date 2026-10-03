package i18n

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"ja", LangJa},
		{"ja_JP.UTF-8", LangJa},
		{"JA", LangJa},
		{"japanese", LangJa},
		{"en", LangEn},
		{"en_US", LangEn},
		{"EN_US.UTF-8", LangEn},
		{"english", LangEn},
		{"fr", LangJa},
		{"fr_FR.UTF-8", LangJa},
		{"C", LangJa},
		{"POSIX", LangJa},
		{"", LangJa},
		{"  ", LangJa},
		{"undetermined", LangJa},
	}
	for _, tt := range tests {
		if got := Resolve(tt.in); got != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolvePriority(t *testing.T) {
	if got := ResolvePriority("", "  ", "fr", "en_US"); got != LangJa {
		t.Errorf("empty values must be skipped: ResolvePriority = %q", got)
	}
	if got := ResolvePriority("en_US", "ja_JP"); got != LangEn {
		t.Errorf("first non-empty wins: ResolvePriority = %q", got)
	}
	if got := ResolvePriority("", "", ""); got != LangJa {
		t.Errorf("all empty falls back to ja: ResolvePriority = %q", got)
	}
	if got := ResolvePriority(); got != LangJa {
		t.Errorf("no values falls back to ja: ResolvePriority = %q", got)
	}
}

func TestResolveFromEnv(t *testing.T) {
	t.Setenv("PROVSYNC_LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "en_US.UTF-8")
	if got := ResolveFromEnv(); got != LangEn {
		t.Errorf("LANG must be used when the higher priority vars are empty: %q", got)
	}
	t.Setenv("LC_MESSAGES", "ja_JP.UTF-8")
	if got := ResolveFromEnv(); got != LangJa {
		t.Errorf("LC_MESSAGES must win over LANG: %q", got)
	}
	t.Setenv("LC_ALL", "en_US")
	if got := ResolveFromEnv(); got != LangEn {
		t.Errorf("LC_ALL must win over LC_MESSAGES: %q", got)
	}
	t.Setenv("PROVSYNC_LANG", "ja")
	if got := ResolveFromEnv(); got != LangJa {
		t.Errorf("PROVSYNC_LANG must win over everything: %q", got)
	}
}

func TestCatalogCompleteness(t *testing.T) {
	jaKeys := catalogKeys(jaCatalog)
	enKeys := catalogKeys(enCatalog)
	if !reflect.DeepEqual(jaKeys, enKeys) {
		t.Errorf("catalog key sets must match:\nja only: %v\nen only: %v",
			diff(jaKeys, enKeys), diff(enKeys, jaKeys))
	}
	for _, id := range jaKeys {
		if strings.TrimSpace(jaCatalog[id]) == "" {
			t.Errorf("ja entry %q must not be empty", id)
		}
		if strings.TrimSpace(enCatalog[id]) == "" {
			t.Errorf("en entry %q must not be empty", id)
		}
		jv, ev := verbs(jaCatalog[id]), verbs(enCatalog[id])
		if !reflect.DeepEqual(jv, ev) {
			t.Errorf("format verbs must match for %q:\nja: %v\nen: %v", id, jv, ev)
		}
	}
}

func TestT(t *testing.T) {
	if got := T(LangJa, "err.write.failed", "/tmp/x %d"); got != "書き込みに失敗しました (/tmp/x %d)" {
		// 値は補間後に % 展開されない(Sprintf の値側は安全)
		t.Errorf("T interpolation: %q", got)
	}
	if got := T(LangEn, "msg.noChanges"); got != "no changes" {
		t.Errorf("T en: %q", got)
	}
	if got := T(LangJa, "msg.pruned", 3); got != "削除: 3 件" {
		t.Errorf("T ja with %d: %q", 3, got)
	}
	if got := T(LangEn, "msg.pruned", 3); got != "removed: 3 entries" {
		t.Errorf("T en with %d: %q", 3, got)
	}
	if got := T(LangJa, "no.such.id"); got != "no.such.id" {
		t.Errorf("unknown ID must return the ID itself: %q", got)
	}
	if got := T(LangEn, "no.such.id"); got != "no.such.id" {
		t.Errorf("unknown ID must return the ID itself: %q", got)
	}
	if got := T("fr", "msg.noChanges"); got != jaCatalog["msg.noChanges"] {
		t.Errorf("unsupported lang must fall back to ja: %q", got)
	}
}

func TestMessageErrorIsJa(t *testing.T) {
	cause := errors.New("underlying")
	m := Wrap(cause, "err.write.failed", "/tmp/x")
	want := "書き込みに失敗しました (/tmp/x): underlying"
	if got := m.Error(); got != want {
		t.Errorf("Message.Error() = %q, want %q", got, want)
	}
	if got := m.Unwrap(); got != cause {
		t.Errorf("Message.Unwrap() = %v, want %v", got, cause)
	}
	if !errors.Is(m, cause) {
		t.Error("errors.Is must find the wrapped cause")
	}
}

func TestLocalize(t *testing.T) {
	// fmt.Errorf("A: %w", fmt.Errorf("B: %w", cause)) と同じ連結形になること
	cause := errors.New("root cause")
	chain := Wrap(Wrap(cause, "err.central.read"), "err.backup.failed")
	if got := Localize(LangJa, chain); got != "バックアップに失敗しました: 中央設定を読めません: root cause" {
		t.Errorf("Localize ja chain = %q", got)
	}
	if got := Localize(LangEn, chain); got != "backup failed: cannot read the central config: root cause" {
		t.Errorf("Localize en chain = %q", got)
	}
	if got := Localize(LangJa, cause); got != "root cause" {
		t.Errorf("plain error must pass through: %q", got)
	}
	if got := Localize(LangJa, nil); got != "" {
		t.Errorf("nil error must render empty: %q", got)
	}
}

func TestLocalizeMatchesFmtErrorf(t *testing.T) {
	// 移行前の fmt.Errorf("…: %w") の描画とバイト等価であること(ja)
	cause := fmt.Errorf("underlying: %w", errors.New("inner"))
	old := fmt.Errorf("書き込みに失敗しました (%s): %w", "/tmp/x", cause).Error()
	neo := MessageErrorForTest(Wrap(cause, "err.write.failed", "/tmp/x"))
	if old != neo {
		t.Errorf("rendering must match fmt.Errorf %%w:\nold: %q\nnew: %q", old, neo)
	}
}

// MessageErrorForTest は (*Message).Error と同じ連結を返すヘルパ。
func MessageErrorForTest(m *Message) string { return m.Error() }

func TestMessageArgsAreSafe(t *testing.T) {
	// args 側に % が含まれても展開されない(fmt.Sprintf の値は安全)
	m := New("err.write.failed", "/tmp/100%s")
	if got := m.Error(); got != "書き込みに失敗しました (/tmp/100%s)" {
		t.Errorf("args must not be format-expanded: %q", got)
	}
}

func catalogKeys(c map[string]string) []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func diff(a, b []string) []string {
	var out []string
	for _, k := range a {
		if !contains(b, k) {
			out = append(out, k)
		}
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

var verbRe = regexp.MustCompile(`%[-+ #0]*[\d]*(?:\.[\d]+)?[a-zA-Z]`)

func verbs(tpl string) []string {
	vs := verbRe.FindAllString(strings.ReplaceAll(tpl, "%%", ""), -1)
	sort.Strings(vs)
	return vs
}
