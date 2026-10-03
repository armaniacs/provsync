package cli

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/i18n"
)

// TestCompletionUnknownShellIsLocalized は未知のシェルの使い方エラーが
// raw のカタログ ID ではなく翻訳済み文言になることを保証する。
func TestCompletionUnknownShellIsLocalized(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "en")
	var out, errOut bytes.Buffer
	err := RunWith([]string{"--root", f.root, "completion", "powershell"}, &out, &errOut)
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("unknown shell must be a *UsageError, got %T: %v", err, err)
	}
	if strings.HasPrefix(ue.Msg, "err.") || strings.Contains(ue.Msg, "unknownShell") {
		t.Errorf("usage error must be localized, not the raw catalog ID: %q", ue.Msg)
	}
	if cjkRe.MatchString(ue.Msg) {
		t.Errorf("en usage error must not contain CJK: %q", ue.Msg)
	}
}

// TestLocalizeStatusVariants は warnings / drift / どちらも無いレポートの
// 表示用複製を検証する。
func TestLocalizeStatusVariants(t *testing.T) {
	tests := []struct {
		name         string
		lang         string
		tool         toolStatus
		wantWarnings []string
		wantDrift    []string
	}{
		{
			name: "warnings but no drift",
			lang: "en",
			tool: toolStatus{
				Name: "kilocode", Path: "/k", Exists: true, Providers: 1,
				warnMsgs: []*i18n.Message{i18n.New("warn.secret.field", "apiKey")},
			},
			wantWarnings: []string{`detected a secret-like field "apiKey"; secrets are never mediated, so it was not imported`},
			wantDrift:    nil,
		},
		{
			name: "drift but no warnings",
			lang: "ja",
			tool: toolStatus{
				Name: "kilocode", Path: "/k", Exists: true,
				DriftEntries: []driftEntry{{Provider: "llm-01", Op: "not-in-tool"}},
			},
			wantWarnings: nil,
			wantDrift:    []string{"llm-01: ツールに無い"},
		},
		{
			name:         "no warnings and no drift keeps nil slices",
			lang:         "en",
			tool:         toolStatus{Name: "kilocode", Path: "/k", Exists: true, Providers: 2},
			wantWarnings: nil,
			wantDrift:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := &statusReport{Tools: []toolStatus{tt.tool}}
			got := localizeStatus(rep, tt.lang)
			if !reflect.DeepEqual(got.Tools[0].Warnings, tt.wantWarnings) {
				t.Errorf("Warnings = %v, want %v", got.Tools[0].Warnings, tt.wantWarnings)
			}
			if !reflect.DeepEqual(got.Tools[0].Drift, tt.wantDrift) {
				t.Errorf("Drift = %v, want %v", got.Tools[0].Drift, tt.wantDrift)
			}
			if got.Tools[0].Name != tt.tool.Name || got.Tools[0].Path != tt.tool.Path ||
				got.Tools[0].Exists != tt.tool.Exists || got.Tools[0].Providers != tt.tool.Providers ||
				!reflect.DeepEqual(got.Tools[0].DriftEntries, tt.tool.DriftEntries) {
				t.Errorf("locale-independent fields changed:\nbefore %+v\nafter  %+v", tt.tool, got.Tools[0])
			}
		})
	}
}

// TestLocalizeStatusDoesNotMutateSource は localizeStatus がソースを
// 破壊しない(複製を返す)ことを保証する。
func TestLocalizeStatusDoesNotMutateSource(t *testing.T) {
	rep := &statusReport{Tools: []toolStatus{{
		Name: "kilocode", Path: "/k", Exists: true,
		warnMsgs:     []*i18n.Message{i18n.New("warn.secret.field", "apiKey")},
		DriftEntries: []driftEntry{{Provider: "p1", Op: "drift"}},
	}}}
	got := localizeStatus(rep, "en")
	if got == rep {
		t.Fatal("localizeStatus must return a copy")
	}
	if rep.Tools[0].Warnings != nil {
		t.Errorf("source Warnings must stay nil, got %v", rep.Tools[0].Warnings)
	}
	if rep.Tools[0].Drift != nil {
		t.Errorf("source Drift must stay nil, got %v", rep.Tools[0].Drift)
	}
	if len(got.Tools[0].Warnings) == 0 || len(got.Tools[0].Drift) == 0 {
		t.Errorf("localized copy must be translated: %+v", got.Tools[0])
	}
}

// TestDriftOpKeyUnknownOp は未登録 op がパニックではなく空ラベルになることを保証する。
func TestDriftOpKeyUnknownOp(t *testing.T) {
	if got := driftOpKey("future-op"); got != "" {
		t.Errorf("unknown op must map to an empty key: %q", got)
	}
	rep := &statusReport{Tools: []toolStatus{{
		Name: "kilocode", Exists: true,
		DriftEntries: []driftEntry{{Provider: "p1", Op: "future-op"}},
	}}}
	got := localizeStatus(rep, "ja")
	want := []string{"p1: "}
	if !reflect.DeepEqual(got.Tools[0].Drift, want) {
		t.Errorf("unknown op must render an empty label: %q, want %q", got.Tools[0].Drift, want)
	}
}

// TestRenderStatusWarningLines は警告が "ラベル: 本文" の形で errOut に
// 出ること(stdout に出ないこと)を ja / en で検証する。
func TestRenderStatusWarningLines(t *testing.T) {
	tests := []struct {
		lang, body, want string
	}{
		{"ja", "検出しました", "  警告: 検出しました\n"},
		{"en", "detected", "  warning: detected\n"},
	}
	for _, tt := range tests {
		var out, errOut bytes.Buffer
		o := &options{out: &out, errOut: &errOut, lang: tt.lang}
		rep := &statusReport{
			Central: centralInfo{Path: "/nonexistent-provsync-central", Exists: true},
			Tools: []toolStatus{{
				Name: "kilocode", Path: "/k", Exists: true, Providers: 1,
				Warnings: []string{tt.body},
			}},
		}
		renderStatusText(o, adapter.ResolveRoot(t.TempDir(), ""), rep)
		if !strings.Contains(errOut.String(), tt.want) {
			t.Errorf("warnings must render with the %s label on errOut:\n%q", tt.lang, errOut.String())
		}
		if strings.Contains(out.String(), "警告") || strings.Contains(out.String(), "warning") {
			t.Errorf("warnings must not go to stdout:\n%s", out.String())
		}
	}
}

// TestCountStatusMixed は集計関数の直値テスト。
func TestCountStatusMixed(t *testing.T) {
	checks := []diagnosisCheck{
		{nameID: "a", status: statusOK},
		{nameID: "b", status: statusOK},
		{nameID: "c", status: statusWarn},
		{nameID: "d", status: statusNG},
	}
	for _, tt := range []struct {
		status checkStatus
		want   int
	}{{statusOK, 2}, {statusWarn, 1}, {statusNG, 1}} {
		if got := countStatus(checks, tt.status); got != tt.want {
			t.Errorf("countStatus(%v) = %d, want %d", tt.status, got, tt.want)
		}
	}
	if got := countStatus(nil, statusOK); got != 0 {
		t.Errorf("countStatus(nil) = %d, want 0", got)
	}
}

// TestOptionsMessageHelpers は o.msgf / o.warnf / o.warnm / o.usageErr の
// 描画を ja / en で検証する。
func TestOptionsMessageHelpers(t *testing.T) {
	tests := []struct {
		lang      string
		wantMsg   string
		wantWarn  string
		wantWarnM string
		wantUsage string
	}{
		{
			lang: "ja", wantMsg: "変更はありません\n", wantWarn: "警告: セントラル設定を読めません\n",
			wantWarnM: "警告: セントラル設定を読めません: disk full\n", wantUsage: "使い方: provsync pull <tool>",
		},
		{
			lang: "en", wantMsg: "no changes\n", wantWarn: "warning: cannot read the central config\n",
			wantWarnM: "warning: cannot read the central config: disk full\n", wantUsage: "usage: provsync pull <tool>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			var out, errOut bytes.Buffer
			o := &options{out: &out, errOut: &errOut, lang: tt.lang}
			o.msgf(o.out, "msg.noChanges")
			if out.String() != tt.wantMsg {
				t.Errorf("msgf = %q, want %q", out.String(), tt.wantMsg)
			}
			o.warnf("err.central.read")
			if errOut.String() != tt.wantWarn {
				t.Errorf("warnf = %q, want %q", errOut.String(), tt.wantWarn)
			}
			errOut.Reset()
			o.warnm(i18n.Wrap(errors.New("disk full"), "err.central.read"))
			if errOut.String() != tt.wantWarnM {
				t.Errorf("warnm = %q, want %q", errOut.String(), tt.wantWarnM)
			}
			target, ok := o.usageErr("err.usage.pull").(*UsageError)
			if !ok || target.Msg != tt.wantUsage {
				t.Errorf("usageErr must localize the message, got %#v", target)
			}
		})
	}
}

// TestDoctorCentralNG はセントラル設定が壊れているときの NG 描画を検証する。
func TestDoctorCentralNG(t *testing.T) {
	f := setup(t)
	write(t, f.central, "{ invalid json")
	out, err := run(t, f.root, "doctor")
	if err == nil {
		t.Fatal("doctor must error when the central config is invalid")
	}
	if !strings.Contains(err.Error(), "diagnosis found problems") {
		t.Errorf("doctor error must summarize: %v", err)
	}
	if !strings.Contains(out, "[NG] セントラル設定: セントラル設定の JSON が不正です") {
		t.Errorf("doctor must render the NG check with the translated detail:\n%s", out)
	}
}

// TestExitErrorMessage は ExitError の契約を固定する。
func TestExitErrorMessage(t *testing.T) {
	if got := (&ExitError{Code: 3}).Error(); got != "exit 3" {
		t.Errorf("ExitError.Error() = %q, want %q", got, "exit 3")
	}
}
