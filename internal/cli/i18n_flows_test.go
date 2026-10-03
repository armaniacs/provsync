package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/armaniacs/provsync/internal/i18n"
)

// TestEnglishMultiStepFlowUndoManifestStable は en ロケールでの
// pull → push → undo → undo --list の一連の流れを検証する。
// 永続化されるマニフェストのコマンドラベルは安定識別子として
// 言語に依存しないことを保証する。
func TestEnglishMultiStepFlowUndoManifestStable(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "en")

	mustRun(t, f.root, "pull", "kilocode", "--write")
	pushOut := mustRun(t, f.root, "push", "opencode", "--write")
	undoOut := mustRun(t, f.root, "undo")
	listOut := mustRun(t, f.root, "undo", "--list")

	for _, want := range []string{"backup: ", "wrote: "} {
		if !strings.Contains(pushOut, want) {
			t.Errorf("push output missing %q:\n%s", want, pushOut)
		}
	}
	if !strings.Contains(undoOut, "restored: ") || !strings.Contains(undoOut, "redo: provsync undo ") {
		t.Errorf("undo output must be English:\n%s", undoOut)
	}
	if !strings.Contains(undoOut, "(push opencode)") {
		t.Errorf("undo must embed the stable ASCII command label:\n%s", undoOut)
	}
	if !strings.Contains(listOut, "pull kilocode") || !strings.Contains(listOut, "push opencode") {
		t.Errorf("undo --list must show stable ASCII labels:\n%s", listOut)
	}
	if got := strings.Count(listOut, " [undone]"); got != 1 {
		t.Errorf("undo --list must mark 1 undone op, got %d:\n%s", got, listOut)
	}

	// 永続化されたマニフェストのコマンドラベルは言語に依存しない
	idx := filepath.Join(f.root, ".local", "state", "provsync", "index.json")
	var manifest struct {
		Operations []struct {
			ID      string `json:"id"`
			Command string `json:"command"`
			Undone  bool   `json:"undone"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(read(t, idx)), &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(manifest.Operations) != 3 {
		t.Fatalf("operations = %d, want 3", len(manifest.Operations))
	}
	ops := manifest.Operations
	if !strings.HasPrefix(ops[0].Command, "undo ") {
		t.Errorf("newest op must be the undo record: %+v", ops[0])
	}
	if ops[1].Command != "push opencode" || !ops[1].Undone {
		t.Errorf("push op label changed: %+v", ops[1])
	}
	if ops[2].Command != "pull kilocode" || ops[2].Undone {
		t.Errorf("pull op label changed: %+v", ops[2])
	}
	for _, op := range ops {
		if cjkRe.MatchString(op.Command) {
			t.Errorf("manifest command must stay ASCII in en runs: %q", op.Command)
		}
	}
	if cjkRe.MatchString(listOut) || cjkRe.MatchString(undoOut) || cjkRe.MatchString(pushOut) {
		t.Error("en flow output must not contain CJK")
	}
}

// TestEnglishInitFlow は init の検出・複数候補・秘密警告・次の手順を en で検証する。
func TestEnglishInitFlow(t *testing.T) {
	t.Run("detected", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		if err := os.Remove(f.opencode); err != nil {
			t.Fatal(err)
		}
		out := mustRun(t, f.root, "init", "--write")
		if !strings.Contains(out, "detected tool: kilocode") {
			t.Errorf("init must report the detected tool in English:\n%s", out)
		}
		if !strings.Contains(out, "next steps:") {
			t.Errorf("init must print next steps in English:\n%s", out)
		}
		if cjkRe.MatchString(out) {
			t.Errorf("en init output must not contain CJK:\n%s", out)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		out := mustRun(t, f.root, "init")
		for _, want := range []string{
			"multiple tool configs were found:",
			"specify a tool with provsync init <tool>",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("init output missing %q:\n%s", want, out)
			}
		}
		if cjkRe.MatchString(out) {
			t.Errorf("en init output must not contain CJK:\n%s", out)
		}
	})

	t.Run("secretHint", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		// ダミー文字列。実シークレットを置かないこと。
		write(t, f.kilo, `{
  "provider": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://a.example/v1", "apiKey": "sk-dummy-test-1234" },
      "models": {}
    }
  }
}`)
		out := mustRun(t, f.root, "init", "kilocode", "--write")
		if !strings.Contains(out, `detected a secret-like field "options.apiKey"`) {
			t.Errorf("init must warn about the plaintext key in English:\n%s", out)
		}
		if !strings.Contains(out, "secrets are not stored in the central config. Set the env var name in apiKeyEnv") {
			t.Errorf("init must print the secret hint in English:\n%s", out)
		}
		if strings.Contains(read(t, f.central), "sk-dummy-test-1234") {
			t.Error("central config must not contain the dummy secret")
		}
		if cjkRe.MatchString(out) {
			t.Errorf("en init output must not contain CJK:\n%s", out)
		}
	})

	t.Run("nextSteps", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		out := mustRun(t, f.root, "init", "kilocode", "--write")
		for _, want := range []string{"next steps:", "provsync status", "provsync push <other-tool>", "provsync undo"} {
			if !strings.Contains(out, want) {
				t.Errorf("next steps block missing %q:\n%s", want, out)
			}
		}
		if cjkRe.MatchString(out) {
			t.Errorf("en init output must not contain CJK:\n%s", out)
		}
	})
}

// TestEnglishPushWarnings は push の routes / aliases の警告を en で検証する。
func TestEnglishPushWarnings(t *testing.T) {
	t.Run("routeUnused", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		write(t, f.central, routeCentral)
		// env 未設定の状態にする。t.Setenv は終了時に復元される。
		t.Setenv("ANTHROPIC_API_KEY", "")
		os.Unsetenv("ANTHROPIC_API_KEY")
		t.Setenv("OPENROUTER_API_KEY", "")
		os.Unsetenv("OPENROUTER_API_KEY")

		out := mustRun(t, f.root, "push", "opencode", "--write")
		if !strings.Contains(out, `no usable provider on the "sonnet" route (apiKeyEnv is not set); leaving providers unchanged`) {
			t.Errorf("route warning must be English:\n%s", out)
		}
		if cjkRe.MatchString(out) {
			t.Errorf("en push output must not contain CJK:\n%s", out)
		}
	})

	t.Run("aliasUndefined", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		write(t, f.central, `{
  "aliases": {
    "sonnet": { "kilocode": "claude-sonnet-4-5" }
  },
  "providers": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "baseURL": "https://a.example/v1",
      "models": { "sonnet": { "name": "Sonnet" } }
    }
  },
  "version": 1
}`)
		out := mustRun(t, f.root, "push", "opencode", "--write")
		if !strings.Contains(out, `alias "sonnet" has no model ID for tool "opencode"; passing the model name through unchanged`) {
			t.Errorf("alias warning must be English:\n%s", out)
		}
		if !strings.Contains(read(t, f.opencode), `"sonnet"`) {
			t.Error("undefined alias must pass the key through as-is")
		}
		if cjkRe.MatchString(out) {
			t.Errorf("en push output must not contain CJK:\n%s", out)
		}
	})
}

// TestEnglishDoctorWithCentral は doctor の en の detail 行を検証する。
func TestEnglishDoctorWithCentral(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "en")
	write(t, f.central, `{
  "providers": {
    "llm-01": {
      "name": "LLM 01",
      "npm": "@ai-sdk/openai-compatible",
      "baseURL": "https://a.example/v1",
      "apiKeyEnv": "PROVSYNC_TEST_DOCTOR_KEY"
    },
    "llm-02": {
      "name": "LLM 02",
      "npm": "@ai-sdk/openai-compatible",
      "baseURL": "https://b.example/v1",
      "apiKeyEnv": "PROVSYNC_TEST_DOCTOR_MISSING"
    }
  },
  "version": 1
}`)
	t.Setenv("PROVSYNC_TEST_DOCTOR_KEY", "dummy-value")
	t.Setenv("PROVSYNC_TEST_DOCTOR_MISSING", "")
	os.Unsetenv("PROVSYNC_TEST_DOCTOR_MISSING")

	out := mustRun(t, f.root, "doctor")
	for _, want := range []string{
		"[OK] central config",
		"[OK] apiKeyEnv llm-01",
		"[warning] apiKeyEnv llm-02: env var PROVSYNC_TEST_DOCTOR_MISSING is not set",
		"diagnosis: ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}
	if cjkRe.MatchString(out) {
		t.Errorf("en doctor output must not contain CJK:\n%s", out)
	}
}

// TestEnglishErrorFlows は実行時エラーの en 描画(main と同じ Localize 経路)を検証する。
func TestEnglishErrorFlows(t *testing.T) {
	t.Run("brokenSymlink", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		mustRun(t, f.root, "pull", "kilocode", "--write")
		if err := os.Remove(f.opencode); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(t.TempDir(), "missing.json"), f.opencode); err != nil {
			t.Fatal(err)
		}

		_, err := run(t, f.root, "push", "opencode", "--write")
		if err == nil {
			t.Fatal("expected error for broken symlink")
		}
		// 実行時エラーは *i18n.Message で、err.Error() は en(正)を返す。
		// en の描画は main と同じ i18n.Localize 経由で検証する。
		got := i18n.Localize("en", err)
		if !strings.Contains(got, "cannot resolve the symlink target") {
			t.Errorf("en error must mention the broken symlink: %v", got)
		}
		if cjkRe.MatchString(got) {
			t.Errorf("en error must not contain CJK: %q", got)
		}
	})

	t.Run("invalidJSONCToolConfig", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		write(t, f.kilo, "{ \"provider\": ")
		_, err := run(t, f.root, "pull", "kilocode")
		if err == nil {
			t.Fatal("expected error for invalid JSONC")
		}
		got := i18n.Localize("en", err)
		if !strings.Contains(got, "invalid JSONC in the config") {
			t.Errorf("en error must wrap err.config.invalid: %v", got)
		}
		if cjkRe.MatchString(got) {
			t.Errorf("en error must not contain CJK: %q", got)
		}
	})

	t.Run("invalidCentralJSON", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		write(t, f.central, "{ not json")
		_, err := run(t, f.root, "status")
		if err == nil {
			t.Fatal("expected error for malformed central config")
		}
		got := i18n.Localize("en", err)
		if !strings.Contains(got, "invalid JSON in the central config") {
			t.Errorf("en error must wrap err.central.invalid: %v", got)
		}
		if cjkRe.MatchString(got) {
			t.Errorf("en error must not contain CJK: %q", got)
		}
	})

	t.Run("undoUnknownID", func(t *testing.T) {
		f := setup(t)
		t.Setenv("PROVSYNC_LANG", "en")
		_, err := run(t, f.root, "undo", "no-such-id")
		if err == nil {
			t.Fatal("expected error for unknown undo ID")
		}
		got := i18n.Localize("en", err)
		if !strings.Contains(got, `operation "no-such-id" not found`) {
			t.Errorf("en error must wrap err.undo.notFound: %v", got)
		}
		if cjkRe.MatchString(got) {
			t.Errorf("en error must not contain CJK: %q", got)
		}
	})
}

// TestLocalePrecedenceRunWith は RunWith での言語判定の優先順位を検証する。
func TestLocalePrecedenceRunWith(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "emptyProvsyncLangFallsToLANG",
			env:  map[string]string{"PROVSYNC_LANG": "", "LC_ALL": "", "LC_MESSAGES": "", "LANG": "en"},
			want: "not created",
		},
		{
			name: "provsyncLangOverridesLANG",
			env:  map[string]string{"PROVSYNC_LANG": "ja", "LC_ALL": "", "LC_MESSAGES": "", "LANG": "en"},
			want: "未作成",
		},
		{
			name: "lcMessagesOverridesLANG",
			env:  map[string]string{"PROVSYNC_LANG": "", "LC_ALL": "", "LC_MESSAGES": "en", "LANG": "ja"},
			want: "not created",
		},
		{
			name: "lcAllOverridesLcMessages",
			env:  map[string]string{"PROVSYNC_LANG": "", "LC_ALL": "en", "LC_MESSAGES": "ja", "LANG": "ja"},
			want: "not created",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			out := mustRun(t, f.root, "list")
			if !strings.Contains(out, tc.want) {
				t.Errorf("env %v: output missing %q:\n%s", tc.env, tc.want, out)
			}
		})
	}
}

// TestCompletionScriptNotLocalized は補完スクリプト本体が機械可読のため
// ja / en で同一であることを保証する。
func TestCompletionScriptNotLocalized(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		t.Run(shell, func(t *testing.T) {
			f := setup(t)
			jaOut := mustRun(t, f.root, "completion", shell)
			t.Setenv("PROVSYNC_LANG", "en")
			enOut := mustRun(t, f.root, "completion", shell)

			if jaOut != enOut {
				t.Errorf("%s completion script must be byte-identical between ja and en", shell)
			}
			for _, want := range []string{"kilocode", "opencode", "status", "undo", "completion"} {
				if !strings.Contains(enOut, want) {
					t.Errorf("%s completion script missing %q:\n%s", shell, want, enOut)
				}
			}
		})
	}
}

// TestUnsupportedLocaleFRStatusJSONFallsBackToEn は未対応ロケール(fr)での
// status --json の drift 値が en にフォールバックすることを検証する。
func TestUnsupportedLocaleFRStatusJSONFallsBackToEn(t *testing.T) {
	f := setup(t)
	t.Setenv("PROVSYNC_LANG", "fr")
	mustRun(t, f.root, "pull", "kilocode", "--write")
	out := mustRun(t, f.root, "status", "--json")

	var rep struct {
		SchemaVersion int `json:"schemaVersion"`
		Central       struct {
			Path      string `json:"path"`
			Exists    bool   `json:"exists"`
			Providers int    `json:"providers"`
		} `json:"central"`
		Tools []struct {
			Name  string   `json:"name"`
			Drift []string `json:"drift"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("status --json is not valid JSON: %v\n%s", err, out)
	}
	if rep.SchemaVersion != 1 || !rep.Central.Exists || rep.Central.Providers != 2 {
		t.Errorf("key structure must be locale independent: %+v", rep)
	}
	// fr は未対応 → drift 値は en にフォールバックする
	driftFound := false
	for _, ts := range rep.Tools {
		for _, line := range ts.Drift {
			driftFound = true
			if cjkRe.MatchString(line) {
				t.Errorf("fr drift line must fall back to en: %q", line)
			}
		}
	}
	if !driftFound {
		t.Fatal("setup must produce drift")
	}
	for _, jaLabel := range []string{"ツールに無い", "中央に無い", "差分あり"} {
		if strings.Contains(out, jaLabel) {
			t.Errorf("fr drift must not use the ja label %q:\n%s", jaLabel, out)
		}
	}
}
