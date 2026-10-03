package i18n

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
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
		{"fr", LangEn},
		{"fr_FR.UTF-8", LangEn},
		{"C", LangEn},
		{"POSIX", LangEn},
		{"", LangEn},
		{"  ", LangEn},
		{"undetermined", LangEn},
	}
	for _, tt := range tests {
		if got := Resolve(tt.in); got != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolvePriority(t *testing.T) {
	if got := ResolvePriority("", "  ", "fr", "en_US"); got != LangEn {
		t.Errorf("empty values must be skipped: ResolvePriority = %q", got)
	}
	if got := ResolvePriority("en_US", "ja_JP"); got != LangEn {
		t.Errorf("first non-empty wins: ResolvePriority = %q", got)
	}
	if got := ResolvePriority("ja_JP", "en_US"); got != LangJa {
		t.Errorf("first non-empty wins: ResolvePriority = %q", got)
	}
	if got := ResolvePriority("", "", ""); got != LangEn {
		t.Errorf("all empty falls back to en: ResolvePriority = %q", got)
	}
	if got := ResolvePriority(); got != LangEn {
		t.Errorf("no values falls back to en: ResolvePriority = %q", got)
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
	if got := T("fr", "msg.noChanges"); got != enCatalog["msg.noChanges"] {
		t.Errorf("unsupported lang must fall back to en: %q", got)
	}
}

func TestMessageErrorIsEn(t *testing.T) {
	cause := errors.New("underlying")
	m := Wrap(cause, "err.write.failed", "/tmp/x")
	want := "failed to write (/tmp/x): underlying"
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
	if got := Localize(LangJa, chain); got != "バックアップに失敗しました: セントラル設定を読めません: root cause" {
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
	// fmt.Errorf("…: %w") の描画とバイト等価であること(en = 既定言語)
	cause := fmt.Errorf("underlying: %w", errors.New("inner"))
	old := fmt.Errorf("failed to write (%s): %w", "/tmp/x", cause).Error()
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
	if got := m.Error(); got != "failed to write (/tmp/100%s)" {
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

// ---- カタログとコードの整合(source scan) ----

// scanCatalogIDRefs はリポジトリの非テスト .go から静的なカタログ ID 参照を
// 抽出する。-trimpath 付きビルドでは動かない(通常の go test 用)。
func scanCatalogIDRefs(t *testing.T) map[string]string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\.T\("([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`(?m)(?:^|[^.\w])(?:i18n\.)?T\([^,()]+,\s*"([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`msgf\([^,]+,\s*"([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`usageErr\("([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`warnf\("([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`(?m)(?:^|[^.\w])(?:i18n\.)?New\("([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`(?m)(?:^|[^.\w])(?:i18n\.)?Wrap\([^,]+,\s*"([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`(?:helpKey|summaryKey):\s*"([a-z][A-Za-z0-9.]*)"`),
		// tui のコマンドメニューは入力ラベル ID を labelKey 付きリテラルで持つ。
		regexp.MustCompile(`labelKey:\s*"([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`nameID:\s*"([a-z][A-Za-z0-9.]*)"`),
		regexp.MustCompile(`"(status\.op\.[A-Za-z]+)"`),
	}
	refs := map[string]string{}
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".kilo", "testdata", "i18n", "site":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, path)
		for _, re := range patterns {
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				refs[m[1]] = rel
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return refs
}

// TestCatalogCoversCodeIDs はコードから参照される全 ID が両カタログに
// 存在することを保証する。未登録 ID は T が raw ID を返すため、
// ユーザーに ID が見える実バグになる。
func TestCatalogCoversCodeIDs(t *testing.T) {
	for id, file := range scanCatalogIDRefs(t) {
		if _, ok := jaCatalog[id]; !ok {
			t.Errorf("ID %q referenced in %s is missing from jaCatalog", id, file)
		}
		if _, ok := enCatalog[id]; !ok {
			t.Errorf("ID %q referenced in %s is missing from enCatalog", id, file)
		}
	}
}

// TestNoOrphanCatalogEntries はカタログに載せているがコードから参照されない
// 孤立エントリを検出する(翻訳の保守コードの遊びを防ぐ)。
func TestNoOrphanCatalogEntries(t *testing.T) {
	refs := scanCatalogIDRefs(t)
	for _, id := range catalogKeys(jaCatalog) {
		if _, ok := refs[id]; !ok {
			t.Errorf("catalog entry %q is not referenced by any non-test code", id)
		}
	}
}

// TestLocalizeMixedMessageChain は fmt.Errorf が間に入るチェーンでも
// 二重訳せず 1 回ずつ翻訳されることを検証する。
func TestLocalizeMixedMessageChain(t *testing.T) {
	cause := errors.New("disk full")
	inner := Wrap(cause, "err.central.read")
	mid := fmt.Errorf("pull failed: %w", inner)
	outer := Wrap(mid, "err.backup.failed")

	wantJa := "バックアップに失敗しました: pull failed: セントラル設定を読めません: disk full"
	if got := Localize(LangJa, outer); got != wantJa {
		t.Errorf("Localize ja mixed chain = %q, want %q", got, wantJa)
	}
	wantEn := "backup failed: pull failed: cannot read the central config: disk full"
	if got := Localize(LangEn, outer); got != wantEn {
		t.Errorf("Localize en mixed chain = %q, want %q", got, wantEn)
	}
	if got := outer.Error(); got != wantEn {
		t.Errorf("Message.Error() mixed chain = %q, want %q (en runtime parity)", got, wantEn)
	}
}

// TestTMultipleArgs は複数引数の補間を検証する。
func TestTMultipleArgs(t *testing.T) {
	tests := []struct {
		lang, id string
		args     []any
		want     string
	}{
		{LangJa, "msg.central", []any{"/c", 2}, "セントラル設定: /c (2 providers)"},
		{LangEn, "msg.central", []any{"/c", 2}, "central config: /c (2 providers)"},
		{LangJa, "warn.loosePerm", []any{0o644, "/a", "600 /a", "/a"}, "権限が緩い (0644): /a  chmod 600 /a /a"},
	}
	for _, tt := range tests {
		if got := T(tt.lang, tt.id, tt.args...); got != tt.want {
			t.Errorf("T(%q, %q, %v) = %q, want %q", tt.lang, tt.id, tt.args, got, tt.want)
		}
	}
}

// TestMessageNilArgs は nil Args と単独 Message の描画を検証する。
func TestMessageNilArgs(t *testing.T) {
	m := New("msg.noChanges")
	if got := m.Error(); got != "no changes" {
		t.Errorf("Message with nil Args: Error() = %q", got)
	}
	if got := Localize(LangEn, m); got != "no changes" {
		t.Errorf("Localize of a single Message = %q", got)
	}
	if err := errors.Unwrap(m); err != nil {
		t.Errorf("Unwrap of New() must be nil: %v", err)
	}
	if Localize(LangEn, m) != m.Error() {
		t.Error("single Message must render identically in Error() and Localize(en)")
	}
}

// TestCatalogTranslationHygiene は en に ja がコピーされたままのエントリを
// 検出する(CJK を含む ja 値と en 値が一致してはならない)。
func TestCatalogTranslationHygiene(t *testing.T) {
	for _, id := range catalogKeys(jaCatalog) {
		jv, ev := jaCatalog[id], enCatalog[id]
		if catalogCJKRe.MatchString(ev) {
			t.Errorf("en entry %q must not contain CJK: %q", id, ev)
		}
		if catalogCJKRe.MatchString(jv) && jv == ev {
			t.Errorf("en entry %q must differ from the ja value: %q", id, ev)
		}
	}
}

var catalogCJKRe = regexp.MustCompile(`[\p{Hiragana}\p{Katakana}\p{Han}]`)

// TestResolveExtraLocaleVariants は表記ゆれの境界を検証する。
func TestResolveExtraLocaleVariants(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{" ja_JP.UTF-8 ", LangJa},
		{"en ", LangEn},
		{"ja-JP", LangJa},
		{"en-GB", LangEn},
		{"e", LangEn},
		{"eng", LangEn},
	}
	for _, tt := range tests {
		if got := Resolve(tt.in); got != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestResolveFromEnvSkipsWhitespaceLang は空白のみの PROVSYNC_LANG が
// 未設定扱いで読み飛ばされることを検証する。
func TestResolveFromEnvSkipsWhitespaceLang(t *testing.T) {
	t.Setenv("PROVSYNC_LANG", "   ")
	t.Setenv("LC_ALL", "en_US")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "ja_JP.UTF-8")
	if got := ResolveFromEnv(); got != LangEn {
		t.Errorf("whitespace-only PROVSYNC_LANG must fall through to LC_ALL: %q", got)
	}
}

// TestLocalizeDeepChainOrder は深いチェーンが外側から順に連結されることを検証する。
func TestLocalizeDeepChainOrder(t *testing.T) {
	chain := Wrap(
		Wrap(
			Wrap(
				Wrap(errors.New("io"), "err.central.read"),
				"err.backup.failed"),
			"err.recordHistory.failed"),
		"err.undo.none")
	want := "undo できる操作がありません: 履歴の記録に失敗しました: バックアップに失敗しました: セントラル設定を読めません: io"
	if got := Localize(LangJa, chain); got != want {
		t.Errorf("deep chain order = %q, want %q", got, want)
	}
}

// TestErrorsAsFindsMessage は errors.As がチェーンの外側の Message を見つけることを検証する。
func TestErrorsAsFindsMessage(t *testing.T) {
	chain := Wrap(fmt.Errorf("io: %w", New("err.central.read")), "err.backup.failed")
	var m *Message
	if !errors.As(chain, &m) {
		t.Fatal("errors.As must find a *Message in the chain")
	}
	if m.ID != "err.backup.failed" {
		t.Errorf("errors.As must find the outermost Message, got %q", m.ID)
	}
	var inner *Message
	if !errors.As(chain.Err, &inner) || inner.ID != "err.central.read" {
		t.Errorf("inner Message must be reachable: %v", chain.Err)
	}
}
