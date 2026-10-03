package cli

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// cjkRe はひらがな・カタカナ・漢字。en ロケール実行時の出力に現れてはならない
// (文字列移行の取りこぼし検知)。symlinkSuffix の "→" は言語中立のため含めない。
var cjkRe = regexp.MustCompile(`[\p{Hiragana}\p{Katakana}\p{Han}]`)

// TestEnglishAllCommands は BDD「英語ロケールのユーザーが英語でメッセージを読める」。
// 全サブコマンドを PROVSYNC_LANG=en で実行し、英語フラグメントの存在と
// 日本語文字の不在を検証する。
func TestEnglishAllCommands(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
		// wantErr が true のときは戻り値のエラー文字列を検査する
		wantErr bool
	}{
		{name: "list", args: []string{"list"}, want: "not created"},
		{name: "status", args: []string{"status"}, want: "not created"},
		{name: "help", args: []string{"--help"}, want: "usage: provsync <command> [flags]"},
		{name: "help-cmd", args: []string{"pull", "--help"}, want: "import a tool config into the central config"},
		{name: "doctor", args: []string{"doctor"}, want: "diagnosis:"},
		{name: "pull-preview", args: []string{"pull", "kilocode"}, want: "preview only; apply with --write"},
		{name: "undo-list", args: []string{"undo", "--list"}, want: "no history"},
		{name: "usage-error", args: []string{"pull"}, want: "usage: provsync pull <tool>", wantErr: true},
		{name: "unknown-command", args: []string{"nope"}, want: "unknown command", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			t.Setenv("PROVSYNC_LANG", "en")
			out, err := run(t, f.root, tc.args...)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %v, want containing %q", err, tc.want)
				}
				if cjkRe.MatchString(err.Error()) {
					t.Errorf("en error must not contain CJK: %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("run %v: %v\n%s", tc.args, err, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("output missing %q:\n%s", tc.want, out)
			}
			if cjkRe.MatchString(out) {
				t.Errorf("en output must not contain CJK:\n%s", out)
			}
		})
	}
}

// TestEnglishStatusJSONNoCJK は BDD「機械可読出力の構造は言語の影響を受けない」。
// drift 値が英語になり、日本語文字が消えること(ロケール連動の仕様)を検証する。
func TestEnglishStatusJSONNoCJK(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	t.Setenv("PROVSYNC_LANG", "en")
	out := mustRun(t, f.root, "status", "--json")
	if cjkRe.MatchString(out) {
		t.Errorf("en status --json must not contain CJK:\n%s", out)
	}
	if !strings.Contains(out, "not in tool") {
		t.Errorf("en drift lines must use the en label:\n%s", out)
	}
}

// TestStatusJSONStructureLocaleIndependent は ja / en の status --json の
// 構造(キー・型・配列長・driftEntries の op 値)が同一であることを検証する。
func TestStatusJSONStructureLocaleIndependent(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")

	jaOut := mustRun(t, f.root, "status", "--json")
	t.Setenv("PROVSYNC_LANG", "en")
	enOut := mustRun(t, f.root, "status", "--json")

	type driftEntryT struct {
		Provider string `json:"provider"`
		Op       string `json:"op"`
	}
	type toolT struct {
		Name         string        `json:"name"`
		Path         string        `json:"path"`
		Exists       bool          `json:"exists"`
		Providers    int           `json:"providers"`
		Warnings     []string      `json:"warnings"`
		Drift        []string      `json:"drift"`
		DriftEntries []driftEntryT `json:"driftEntries"`
	}
	type centralT struct {
		Exists    bool `json:"exists"`
		Providers int  `json:"providers"`
	}
	var jaRep, enRep struct {
		SchemaVersion int      `json:"schemaVersion"`
		Central       centralT `json:"central"`
		Tools         []toolT  `json:"tools"`
	}
	if err := json.Unmarshal([]byte(jaOut), &jaRep); err != nil {
		t.Fatalf("ja status --json is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(enOut), &enRep); err != nil {
		t.Fatalf("en status --json is not valid JSON: %v", err)
	}
	if jaRep.SchemaVersion != enRep.SchemaVersion {
		t.Errorf("schemaVersion: ja %d / en %d", jaRep.SchemaVersion, enRep.SchemaVersion)
	}
	if jaRep.Central != enRep.Central {
		t.Errorf("central must be locale independent: ja %+v / en %+v", jaRep.Central, enRep.Central)
	}
	if len(jaRep.Tools) != len(enRep.Tools) {
		t.Fatalf("tools length: ja %d / en %d", len(jaRep.Tools), len(enRep.Tools))
	}
	for i := range jaRep.Tools {
		jt, et := jaRep.Tools[i], enRep.Tools[i]
		if jt.Name != et.Name || jt.Path != et.Path || jt.Exists != et.Exists || jt.Providers != et.Providers {
			t.Errorf("tool %d fields must be locale independent:\nja %+v\nen %+v", i, jt, et)
		}
		if !reflect.DeepEqual(jt.DriftEntries, et.DriftEntries) {
			t.Errorf("driftEntries must be locale independent:\nja %+v\nen %+v", jt.DriftEntries, et.DriftEntries)
		}
		for _, line := range et.Drift {
			if cjkRe.MatchString(line) {
				t.Errorf("en drift line must not contain CJK: %q", line)
			}
		}
		for _, w := range et.Warnings {
			if cjkRe.MatchString(w) {
				t.Errorf("en warning must not contain CJK: %q", w)
			}
		}
	}
	driftFound := false
	for i := range jaRep.Tools {
		if len(jaRep.Tools[i].Drift) == 0 {
			continue
		}
		driftFound = true
		for _, line := range jaRep.Tools[i].Drift {
			if !cjkRe.MatchString(line) {
				t.Errorf("ja drift line must contain CJK (locale-linked): %q", line)
			}
		}
	}
	if !driftFound {
		t.Fatal("setup must produce drift to verify locale-linked values")
	}
}

// TestUnsupportedLocaleFallsBackToEn は BDD「未対応ロケールはエラーにならず英語で動く」。
func TestUnsupportedLocaleFallsBackToEn(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "fr")
	out := mustRun(t, f.root, "list")
	if !strings.Contains(out, "not created") {
		t.Errorf("fr must fall back to en without an error:\n%s", out)
	}
}

// TestUnsetEnvIsEnglish は BDD「既定では英語で表示される」。
func TestUnsetEnvIsEnglish(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	out := mustRun(t, f.root, "list")
	if !strings.Contains(out, "not created") {
		t.Errorf("unset env vars must fall back to the en output:\n%s", out)
	}
}
