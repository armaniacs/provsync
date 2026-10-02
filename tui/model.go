package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// statusReport は `provsync status --json` の出力(schemaVersion: 1)。
// コアの internal を import できないため、最小限の構造をここで定義する。
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
// provider 一覧は drift と warnings からは取れないため、各ツールの provider 数だけが
// 分かる。選択肢は `provsync list --json` ではなく `provsync status --json` の
// tools[i].drift の provider 名から組み立てる(ツールに無い/差分ありのみ候補にする)。
func newSelection(report statusReport) *selection {
	s := &selection{Selected: map[pair]bool{}}
	for _, ts := range report.Tools {
		if !ts.Exists {
			continue
		}
		for _, line := range ts.Drift {
			name, ok := providerFromDriftLine(line)
			if !ok {
				continue
			}
			p := pair{Tool: ts.Name, Provider: name}
			if _, dup := s.Selected[p]; dup {
				continue
			}
			s.Items = append(s.Items, p)
			s.Selected[p] = false
		}
	}
	return s
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
func fetchStatus(bin string) (statusReport, error) {
	out, err := exec.Command(bin, "status", "--json").Output()
	if err != nil {
		return statusReport{}, fmt.Errorf("provsync status --json を実行できません (%s): %w", bin, err)
	}
	var rep statusReport
	if err := json.Unmarshal(out, &rep); err != nil {
		return statusReport{}, fmt.Errorf("provsync status --json の出力が不正です: %w", err)
	}
	if rep.SchemaVersion != 1 {
		return statusReport{}, fmt.Errorf("対応していない schemaVersion です: %d", rep.SchemaVersion)
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
