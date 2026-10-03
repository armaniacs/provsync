// Package cli のうち、list と status を担う。
package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/store"
)

// ---- list ----

func cmdList(o *options, args []string) error {
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
	for _, st := range adapter.CollectToolStates(root, adapter.Names()) {
		if st.Err != nil {
			return st.Err
		}
		ti := toolInfo{Name: st.Name, Path: st.Path}
		if !st.Exists {
			rep.Tools = append(rep.Tools, ti)
			continue
		}
		ti.Exists = true
		ti.Providers = len(st.Providers)
		rep.Tools = append(rep.Tools, ti)
	}
	if o.jsonOut {
		return encodeJSON(o.out, rep)
	}
	if rep.Central.Exists {
		o.msgf(o.out, "msg.central", rep.Central.Path, rep.Central.Providers)
	} else {
		o.msgf(o.out, "msg.centralMissing", rep.Central.Path)
	}
	for _, ti := range rep.Tools {
		if !ti.Exists {
			o.msgf(o.out, "msg.toolMissing", ti.Name, ti.Path)
			continue
		}
		suffix := symlinkSuffix(ti.Path)
		o.msgf(o.out, "msg.tool", ti.Name, ti.Path, suffix, ti.Providers)
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
	// 表示(JSON の値を含む)はロケール連動。--exit-code は機械値で判定する。
	lrep := localizeStatus(rep, o.lang)
	if o.jsonOut {
		if err := encodeJSON(o.out, lrep); err != nil {
			return err
		}
	} else {
		renderStatusText(o, root, lrep)
	}
	if o.exitCode {
		for _, ts := range rep.Tools {
			if len(ts.DriftEntries) > 0 {
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
	// warnMsgs は警告の言語中立の値。JSON には出さず、localizeStatus が
	// Warnings へ翻訳して載せる。
	warnMsgs []*i18n.Message `json:"-"`
}

// driftEntry は 1 provider の差分を構造化したもの。Op の値は固定集合
// (not-in-tool / not-in-central / drift)で、後から勝手に増やさない。
type driftEntry struct {
	Provider string `json:"provider"`
	Op       string `json:"op"`
}

// buildStatusReport は tools とセントラル設定の同期状態を集計する。
// 警告と差分は言語中立の値(warnMsgs / DriftEntries)で保持し、
// 表示(localizeStatus)で初めて言語が決まる。空 = 差分なし。
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

	for _, st := range adapter.CollectToolStates(root, tools) {
		if st.Err != nil {
			return nil, st.Err
		}
		ts := toolStatus{Name: st.Name, Path: st.Path}
		if !st.Exists {
			rep.Tools = append(rep.Tools, ts)
			continue
		}
		ts.Exists = true
		ts.Providers = len(st.Providers)
		ts.warnMsgs = st.Warnings
		if central != nil {
			// Project needs the adapter; Get is a pure constructor
			// that cannot fail for names the collector accepted.
			a, err := adapter.Get(st.Name, root)
			if err != nil {
				return nil, err
			}
			ts.DriftEntries = driftEntries(a.Project(central.Providers), st.Providers)
		}
		rep.Tools = append(rep.Tools, ts)
	}
	return rep, nil
}

// localizeStatus は機械値レポートを表示用(翻訳済み)に複製する。
// JSON のキー・型は不変で、drift 行と warnings の値だけがロケール連動になる。
func localizeStatus(rep *statusReport, lang string) *statusReport {
	out := *rep
	out.Tools = make([]toolStatus, len(rep.Tools))
	for i, ts := range rep.Tools {
		nts := ts
		nts.Warnings = nil
		for _, m := range ts.warnMsgs {
			nts.Warnings = append(nts.Warnings, i18n.Localize(lang, m))
		}
		nts.Drift = nil
		for _, e := range ts.DriftEntries {
			nts.Drift = append(nts.Drift, fmt.Sprintf("%s: %s", e.Provider, i18n.T(lang, driftOpKey(e.Op))))
		}
		out.Tools[i] = nts
	}
	return &out
}

// renderStatusText は statusReport を従来のテキスト形式で出す。
func renderStatusText(o *options, root adapter.Root, rep *statusReport) {
	if rep.Central.Exists {
		o.msgf(o.out, "msg.central", rep.Central.Path, rep.Central.Providers)
		warnLoosePerm(o, rep.Central.Path, false)
	} else {
		o.msgf(o.out, "msg.centralMissing", rep.Central.Path)
	}
	warnLoosePerm(o, root.StateDir(), true)

	for _, ts := range rep.Tools {
		if !ts.Exists {
			o.msgf(o.out, "msg.toolMissing", ts.Name, ts.Path)
			continue
		}
		o.msgf(o.out, "msg.tool", ts.Name, ts.Path, "", ts.Providers)
		for _, w := range ts.Warnings {
			fmt.Fprintf(o.errOut, "  %s: %s\n", o.T("label.warning"), w)
		}
		if !rep.Central.Exists {
			continue
		}
		if len(ts.Drift) == 0 {
			o.msgf(o.out, "msg.noDrift")
			continue
		}
		for _, line := range ts.Drift {
			fmt.Fprintf(o.out, "  %s\n", line)
		}
	}
}

// driftEntries は projected(ツール可視の形へ写したセントラル設定)と tool の
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

// driftOpKey は driftEntry の op を status 表示用のカタログ ID に対応づける。
func driftOpKey(op string) string {
	switch op {
	case "not-in-tool":
		return "status.op.notInTool"
	case "not-in-central":
		return "status.op.notInCentral"
	case "drift":
		return "status.op.drift"
	}
	return ""
}
