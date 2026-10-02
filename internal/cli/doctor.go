// Package cli のうち、doctor 診断を担う。
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/store"
)

// ---- doctor ----

// diagnosisCheck は doctor の 1 診断項目。
type diagnosisCheck struct {
	Name   string
	Status string // OK / 警告 / NG
	Detail string // 次にやること。OK のときは空
}

// cmdDoctor は設定・環境を診断する。通信せず、ファイルも書かない。
// NG が 1 件以上のときはエラー(終了コード 1)で終わる。警告のみなら成功。
func cmdDoctor(o *options) error {
	root, err := o.root()
	if err != nil {
		return err
	}

	var checks []diagnosisCheck
	checks = append(checks, doctorCheckPaths(root)...)
	checks = append(checks, doctorCheckCentral(root)...)
	checks = append(checks, doctorCheckTools(root)...)
	checks = append(checks, doctorCheckPerms(root)...)

	ng := 0
	for _, c := range checks {
		switch c.Status {
		case "OK":
			fmt.Fprintf(o.out, "[OK] %s\n", c.Name)
		case "警告":
			fmt.Fprintf(o.out, "[警告] %s: %s\n", c.Name, c.Detail)
		case "NG":
			fmt.Fprintf(o.out, "[NG] %s: %s\n", c.Name, c.Detail)
			ng++
		}
	}
	fmt.Fprintf(o.out, "診断結果: OK %d 件 / 警告 %d 件 / NG %d 件\n", countStatus(checks, "OK"), countStatus(checks, "警告"), ng)
	if ng > 0 {
		return fmt.Errorf("診断で問題が見つかりました (NG %d 件)", ng)
	}
	return nil
}

func countStatus(checks []diagnosisCheck, status string) int {
	n := 0
	for _, c := range checks {
		if c.Status == status {
			n++
		}
	}
	return n
}

func doctorCheckPaths(root adapter.Root) []diagnosisCheck {
	return []diagnosisCheck{
		{Name: "パス解決", Status: "OK", Detail: root.CentralConfigPath()},
	}
}

func doctorCheckCentral(root adapter.Root) []diagnosisCheck {
	var checks []diagnosisCheck
	centralPath := root.CentralConfigPath()
	if _, err := os.Stat(centralPath); err != nil {
		checks = append(checks, diagnosisCheck{Name: "中央設定", Status: "警告", Detail: "未作成です。provsync init <tool> --write を実行してください"})
		return checks
	}
	cfg, err := store.Load(centralPath)
	if err != nil {
		checks = append(checks, diagnosisCheck{Name: "中央設定", Status: "NG", Detail: err.Error()})
		return checks
	}
	checks = append(checks, diagnosisCheck{Name: "中央設定", Status: "OK", Detail: fmt.Sprintf("%d providers", len(cfg.Providers))})
	checks = append(checks, doctorCheckAPIKeyEnv(cfg)...)
	return checks
}

func doctorCheckAPIKeyEnv(cfg *model.Config) []diagnosisCheck {
	var checks []diagnosisCheck
	keys := make([]string, 0, len(cfg.Providers))
	for k := range cfg.Providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env := cfg.Providers[k].APIKeyEnv
		if env == "" {
			continue
		}
		if _, ok := os.LookupEnv(env); ok {
			checks = append(checks, diagnosisCheck{Name: "apiKeyEnv " + k, Status: "OK", Detail: env})
		} else {
			// 変数の値は読んでも出力しない。名前のみを案内する。
			checks = append(checks, diagnosisCheck{Name: "apiKeyEnv " + k, Status: "警告", Detail: fmt.Sprintf("環境変数 %s が未設定です", env)})
		}
	}
	return checks
}

func doctorCheckTools(root adapter.Root) []diagnosisCheck {
	var checks []diagnosisCheck
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			continue
		}
		if _, err := os.Stat(a.Path()); err != nil {
			checks = append(checks, diagnosisCheck{Name: "ツール " + name, Status: "警告", Detail: fmt.Sprintf("%s が未作成です", a.Path())})
			continue
		}
		_, warnings, err := a.Pull()
		if err != nil {
			checks = append(checks, diagnosisCheck{Name: "ツール " + name, Status: "NG", Detail: err.Error()})
			continue
		}
		if len(warnings) > 0 {
			// pull の警告は秘密らしいキー名のみを含む(値は含まない)。
			checks = append(checks, diagnosisCheck{Name: "ツール " + name, Status: "警告", Detail: strings.Join(warnings, "; ")})
			continue
		}
		checks = append(checks, diagnosisCheck{Name: "ツール " + name, Status: "OK", Detail: a.Path()})
	}
	return checks
}

func doctorCheckPerms(root adapter.Root) []diagnosisCheck {
	var checks []diagnosisCheck
	centralPath := root.CentralConfigPath()
	if info, err := os.Stat(centralPath); err == nil && info.Mode().Perm()&0o077 != 0 {
		checks = append(checks, diagnosisCheck{Name: "権限", Status: "警告", Detail: fmt.Sprintf("権限が緩い (%04o): %s  chmod 600 %s", info.Mode().Perm(), centralPath, centralPath)})
	}
	if stateDir := root.StateDir(); filePermLoose(stateDir) {
		checks = append(checks, diagnosisCheck{Name: "権限", Status: "警告", Detail: fmt.Sprintf("状態ディレクトリの権限が緩い: %s  chmod 700 %s", stateDir, stateDir)})
	}
	return checks
}

func filePermLoose(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().Perm()&0o077 != 0
}
