package main

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/armaniacs/provsync/internal/i18n"
)

// statusReport は `provsync status --json` の出力(schemaVersion: 1)。
// コアの model 構造を import しないため、最小限の構造をここで定義する。
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
	// DriftEntries は drift の構造化版(additive)。旧 CLI の応答には無いため、
	// 存在すればそれを優先し、無ければ drift 文字列の逆解析にフォールバックする。
	DriftEntries []driftEntry `json:"driftEntries,omitempty"`
}

// driftEntry は status --json の driftEntries の 1 要素。op は固定集合
// (not-in-tool / not-in-central / drift)。
type driftEntry struct {
	Provider string `json:"provider"`
	Op       string `json:"op"`
}

// pair は同期対象の 1 組み合わせ(tool × provider)。
type pair struct {
	Tool     string
	Provider string
}

// selection はチェックボックスの選択状態。表示(bubbletea)と分離して単体テスト可能にする。
type selection struct {
	Items    []pair
	Selected map[pair]bool
}

// newSelection は toolStatus から、存在するツールの (tool, provider) 組み合わせを作る。
// driftEntries があればそれを優先する。無い場合(旧 CLI の応答)は drift 文字列の
// 逆解析にフォールバックする。drift 行はロケールで文言が変わるが、逆解析は
// ": " の前の provider 名だけを取り出すため言語に依存しない。
func newSelection(report statusReport) *selection {
	s := &selection{Selected: map[pair]bool{}}
	for _, ts := range report.Tools {
		if !ts.Exists {
			continue
		}
		if len(ts.DriftEntries) > 0 {
			for _, e := range ts.DriftEntries {
				s.addPair(ts.Name, e.Provider)
			}
			continue
		}
		for _, line := range ts.Drift {
			name, ok := providerFromDriftLine(line)
			if !ok {
				continue
			}
			s.addPair(ts.Name, name)
		}
	}
	return s
}

// addPair は重複を避けて候補を追加する。
func (s *selection) addPair(tool, provider string) {
	p := pair{Tool: tool, Provider: provider}
	if _, dup := s.Selected[p]; dup {
		return
	}
	s.Items = append(s.Items, p)
	s.Selected[p] = false
}

// providerFromDriftLine は drift 行("xxx: 中央に無い" など)から provider 名を取り出す。
func providerFromDriftLine(line string) (string, bool) {
	i := strings.Index(line, ": ")
	if i <= 0 {
		return "", false
	}
	return line[:i], true
}

// Toggle は i 番目の選択を切り替える。
func (s *selection) Toggle(i int) {
	if i < 0 || i >= len(s.Items) {
		return
	}
	p := s.Items[i]
	s.Selected[p] = !s.Selected[p]
}

// SelectedPairs は選択済みの組み合わせを返す。
func (s *selection) SelectedPairs() []pair {
	var out []pair
	for _, p := range s.Items {
		if s.Selected[p] {
			out = append(out, p)
		}
	}
	return out
}

// applyArgs は選択済みの組み合わせを provsync 子プロセスの引数列に変換する。
// 変更内容の計算は子プロセス(provsync push)に任せる。
func (s *selection) applyArgs() [][]string {
	var out [][]string
	for _, p := range s.SelectedPairs() {
		out = append(out, []string{"push", p.Tool, "--provider", p.Provider, "--write"})
	}
	return out
}

// fetchStatus は provsync 子プロセスから status --json を取得してパースする。
// エラーは言語中立の Message で返し、描画(main の Localize)が言語を決める。
func fetchStatus(bin string) (statusReport, error) {
	out, err := exec.Command(bin, "status", "--json").Output()
	if err != nil {
		return statusReport{}, i18n.Wrap(err, "err.tui.fetch", bin)
	}
	var rep statusReport
	if err := json.Unmarshal(out, &rep); err != nil {
		return statusReport{}, i18n.Wrap(err, "err.tui.invalidOutput")
	}
	if rep.SchemaVersion != 1 {
		return statusReport{}, i18n.New("err.tui.schema", rep.SchemaVersion)
	}
	return rep, nil
}

// runProvsync は provsync 子プロセスを実行し、stdout と stderr を返す。
func runProvsync(bin string, args ...string) (string, string, error) {
	cmd := exec.Command(bin, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
