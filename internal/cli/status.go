// Package cli のうち、list と status を担う。
package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/store"
)

// ---- list ----

func cmdList(o *options) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	rep := listReport{SchemaVersion: 1, Tools: []toolInfo{}}
	central := root.CentralConfigPath()
	if cfg, err := store.Load(central); err == nil {
		rep.Central = centralInfo{Path: central, Exists: true, Providers: len(cfg.Providers)}
	} else if _, statErr := os.Stat(central); os.IsNotExist(statErr) {
		rep.Central = centralInfo{Path: central, Exists: false}
	} else {
		return err
	}
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			return err
		}
		ti := toolInfo{Name: name, Path: a.Path()}
		if _, err := os.Stat(a.Path()); err != nil {
			ti.Exists = false
			rep.Tools = append(rep.Tools, ti)
			continue
		}
		ti.Exists = true
		providers, _, err := a.Pull()
		if err != nil {
			return err
		}
		ti.Providers = len(providers)
		rep.Tools = append(rep.Tools, ti)
	}
	if o.jsonOut {
		return encodeJSON(o.out, rep)
	}
	if rep.Central.Exists {
		fmt.Fprintf(o.out, "中央設定: %s (%d providers)\n", rep.Central.Path, rep.Central.Providers)
	} else {
		fmt.Fprintf(o.out, "中央設定: %s (未作成)\n", rep.Central.Path)
	}
	for _, ti := range rep.Tools {
		if !ti.Exists {
			fmt.Fprintf(o.out, "%-9s %s (未作成)\n", ti.Name, ti.Path)
			continue
		}
		suffix := symlinkSuffix(ti.Path)
		fmt.Fprintf(o.out, "%-9s %s%s (%d providers)\n", ti.Name, ti.Path, suffix, ti.Providers)
	}
	return nil
}

// listReport は list の出力。--json で使う。
type listReport struct {
	SchemaVersion int         `json:"schemaVersion"`
	Central       centralInfo `json:"central"`
	Tools         []toolInfo  `json:"tools"`
}

type toolInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Providers int    `json:"providers"`
}

// symlinkSuffix は path がシンボリックリンクなら実体を示す接尾辞を返す。
func symlinkSuffix(path string) string {
	li, err := os.Lstat(path)
	if err != nil || li.Mode()&os.ModeSymlink == 0 {
		return ""
	}
	real, err := os.Readlink(path)
	if err != nil {
		return " (symlink)"
	}
	return " (symlink → " + real + ")"
}

// ---- status ----

func cmdStatus(o *options, args []string) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	tools := args
	if len(tools) == 0 {
		tools = adapter.Names()
	}
	rep, err := buildStatusReport(root, tools)
	if err != nil {
		return err
	}
	if o.jsonOut {
		if err := encodeJSON(o.out, rep); err != nil {
			return err
		}
	} else {
		renderStatusText(o, root, rep)
	}
	if o.exitCode {
		for _, ts := range rep.Tools {
			if len(ts.Drift) > 0 {
				return &ExitError{Code: 3}
			}
		}
	}
	return nil
}

// statusReport は status の集計結果。テキスト出力と --json の両方の情報源。
type statusReport struct {
	SchemaVersion int          `json:"schemaVersion"`
	Central       centralInfo  `json:"central"`
	Tools         []toolStatus `json:"tools"`
}

type centralInfo struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Providers int    `json:"providers"`
}

type toolStatus struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Exists    bool     `json:"exists"`
	Providers int      `json:"providers"`
	Warnings  []string `json:"warnings"`
	Drift     []string `json:"drift"`
	// DriftEntries は drift の構造化版(additive)。TUI が文言の逆解析に頼らず
	// 済むようにする。差分がないときは省略する。
	DriftEntries []driftEntry `json:"driftEntries,omitempty"`
}

// driftEntry は 1 provider の差分を構造化したもの。Op の値は固定集合
// (not-in-tool / not-in-central / drift)で、後から勝手に増やさない。
type driftEntry struct {
	Provider string `json:"provider"`
	Op       string `json:"op"`
}

// buildStatusReport は tools と中央設定の同期状態を集計する。
// Drift は driftLines の「差分なし」を除いたもので、空 = 差分なし。
func buildStatusReport(root adapter.Root, tools []string) (*statusReport, error) {
	rep := &statusReport{SchemaVersion: 1, Tools: []toolStatus{}}
	centralPath := root.CentralConfigPath()
	var central *model.Config
	if cfg, err := store.Load(centralPath); err == nil {
		central = cfg
		rep.Central = centralInfo{Path: centralPath, Exists: true, Providers: len(cfg.Providers)}
	} else if _, statErr := os.Stat(centralPath); os.IsNotExist(statErr) {
		rep.Central = centralInfo{Path: centralPath, Exists: false}
	} else {
		return nil, err
	}

	for _, name := range tools {
		a, err := adapter.Get(name, root)
		if err != nil {
			return nil, err
		}
		ts := toolStatus{Name: name, Path: a.Path()}
		if _, err := os.Stat(a.Path()); err != nil {
			rep.Tools = append(rep.Tools, ts)
			continue
		}
		ts.Exists = true
		providers, warnings, err := a.Pull()
		if err != nil {
			return nil, err
		}
		ts.Providers = len(providers)
		ts.Warnings = warnings
		if central != nil {
			entries := driftEntries(a.Project(central.Providers), providers)
			for _, e := range entries {
				ts.DriftEntries = append(ts.DriftEntries, e)
				ts.Drift = append(ts.Drift, fmt.Sprintf("%s: %s", e.Provider, driftOpLabel(e.Op)))
			}
		}
		rep.Tools = append(rep.Tools, ts)
	}
	return rep, nil
}

// renderStatusText は statusReport を従来のテキスト形式で出す。
func renderStatusText(o *options, root adapter.Root, rep *statusReport) {
	if rep.Central.Exists {
		fmt.Fprintf(o.out, "中央設定: %s (%d providers)\n", rep.Central.Path, rep.Central.Providers)
		warnLoosePerm(o.errOut, rep.Central.Path, false)
	} else {
		fmt.Fprintf(o.out, "中央設定: %s (未作成)\n", rep.Central.Path)
	}
	warnLoosePerm(o.errOut, root.StateDir(), true)

	for _, ts := range rep.Tools {
		if !ts.Exists {
			fmt.Fprintf(o.out, "%-9s %s (未作成)\n", ts.Name, ts.Path)
			continue
		}
		fmt.Fprintf(o.out, "%-9s %s (%d providers)\n", ts.Name, ts.Path, ts.Providers)
		for _, w := range ts.Warnings {
			fmt.Fprintf(o.errOut, "  警告: %s\n", w)
		}
		if !rep.Central.Exists {
			continue
		}
		if len(ts.Drift) == 0 {
			fmt.Fprintln(o.out, "  差分なし")
			continue
		}
		for _, line := range ts.Drift {
			fmt.Fprintf(o.out, "  %s\n", line)
		}
	}
}

// driftEntries は projected(ツール可視の形へ写した中央設定)と tool の
// provider 集合を比較し、1 provider ごとの差分を構造化して返す。
func driftEntries(projected, tool map[string]model.Provider) []driftEntry {
	seen := map[string]bool{}
	var keys []string
	for k := range projected {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range tool {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var entries []driftEntry
	for _, k := range keys {
		c, inCentral := projected[k]
		t, inTool := tool[k]
		switch {
		case inCentral && !inTool:
			entries = append(entries, driftEntry{Provider: k, Op: "not-in-tool"})
		case !inCentral && inTool:
			entries = append(entries, driftEntry{Provider: k, Op: "not-in-central"})
		case len(plan.ChangedFields(c, t)) > 0:
			entries = append(entries, driftEntry{Provider: k, Op: "drift"})
		}
	}
	return entries
}

// driftOpLabel は driftEntry の op を status 表示用の文言に対応づける。
func driftOpLabel(op string) string {
	switch op {
	case "not-in-tool":
		return "ツールに無い"
	case "not-in-central":
		return "中央に無い"
	case "drift":
		return "差分あり"
	}
	return ""
}
