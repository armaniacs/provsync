// Package cli のうち、doctor 診断を担う。
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/store"
)

// ---- doctor ----

// checkStatus は診断結果の内部判別子。表示文言は doctor.status.* カタログで決める。
type checkStatus string

const (
	statusOK   checkStatus = "ok"
	statusWarn checkStatus = "warn"
	statusNG   checkStatus = "ng"
)

// diagnosisCheck は doctor の 1 診断項目。name と detail はカタログ ID で持ち、
// 表示(cmdDoctor)で初めて言語が決まる。
type diagnosisCheck struct {
	nameID  string
	nameArg any
	status  checkStatus
	detail  error // nil のとき空。*i18n.Message または下位パッケージのエラー
}

// cmdDoctor は設定・環境を診断する。通信せず、ファイルも書かない。
// NG が 1 件以上のときはエラー(終了コード 1)で終わる。警告のみなら成功。
func cmdDoctor(o *options, args []string) error {
	root, err := o.root()
	if err != nil {
		return err
	}

	var checks []diagnosisCheck
	checks = append(checks, doctorCheckPaths()...)
	checks = append(checks, doctorCheckCentral(root)...)
	checks = append(checks, doctorCheckTools(o.lang, root)...)
	checks = append(checks, doctorCheckPerms(root)...)

	ng := 0
	for _, c := range checks {
		name := o.checkName(c)
		detail := ""
		if c.detail != nil {
			detail = i18n.Localize(o.lang, c.detail)
		}
		switch c.status {
		case statusOK:
			fmt.Fprintf(o.out, "[%s] %s\n", o.T("doctor.status.ok"), name)
		case statusWarn:
			fmt.Fprintf(o.out, "[%s] %s: %s\n", o.T("doctor.status.warn"), name, detail)
		case statusNG:
			fmt.Fprintf(o.out, "[%s] %s: %s\n", o.T("doctor.status.ng"), name, detail)
			ng++
		}
	}
	o.msgf(o.out, "doctor.summary", countStatus(checks, statusOK), countStatus(checks, statusWarn), ng)
	if ng > 0 {
		return i18n.New("err.doctor.problems", ng)
	}
	return nil
}

func countStatus(checks []diagnosisCheck, status checkStatus) int {
	n := 0
	for _, c := range checks {
		if c.status == status {
			n++
		}
	}
	return n
}

// checkName は診断項目の表示名を実行時言語で返す。
// nameArg が nil のときは引数なしの文言を使う(Sprintf の EXTRA を避けるため)。
func (o *options) checkName(c diagnosisCheck) string {
	if c.nameArg == nil {
		return o.T(c.nameID)
	}
	return o.T(c.nameID, c.nameArg)
}

func doctorCheckPaths() []diagnosisCheck {
	return []diagnosisCheck{
		{nameID: "doctor.name.paths", status: statusOK},
	}
}

func doctorCheckCentral(root adapter.Root) []diagnosisCheck {
	var checks []diagnosisCheck
	centralPath := root.CentralConfigPath()
	if _, err := os.Stat(centralPath); err != nil {
		checks = append(checks, diagnosisCheck{nameID: "doctor.name.central", status: statusWarn, detail: i18n.New("doctor.detail.centralMissing")})
		return checks
	}
	cfg, err := store.Load(centralPath)
	if err != nil {
		checks = append(checks, diagnosisCheck{nameID: "doctor.name.central", status: statusNG, detail: err})
		return checks
	}
	checks = append(checks, diagnosisCheck{nameID: "doctor.name.central", status: statusOK})
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
			checks = append(checks, diagnosisCheck{nameID: "doctor.name.apiKeyEnv", nameArg: k, status: statusOK})
		} else {
			// 変数の値は読んでも出力しない。名前のみを案内する。
			checks = append(checks, diagnosisCheck{nameID: "doctor.name.apiKeyEnv", nameArg: k, status: statusWarn, detail: i18n.New("doctor.detail.envMissing", env)})
		}
	}
	return checks
}

func doctorCheckTools(lang string, root adapter.Root) []diagnosisCheck {
	var checks []diagnosisCheck
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			continue
		}
		if _, err := os.Stat(a.Path()); err != nil {
			checks = append(checks, diagnosisCheck{nameID: "doctor.name.tool", nameArg: name, status: statusWarn, detail: i18n.New("doctor.detail.toolMissing", a.Path())})
			continue
		}
		_, warnings, err := a.Pull()
		if err != nil {
			checks = append(checks, diagnosisCheck{nameID: "doctor.name.tool", nameArg: name, status: statusNG, detail: err})
			continue
		}
		if len(warnings) > 0 {
			// pull の警告は秘密らしいキー名のみを含む(値は含まない)。
			parts := make([]string, 0, len(warnings))
			for _, w := range warnings {
				parts = append(parts, i18n.Localize(lang, w))
			}
			checks = append(checks, diagnosisCheck{nameID: "doctor.name.tool", nameArg: name, status: statusWarn, detail: i18n.New("doctor.detail.toolWarnings", strings.Join(parts, "; "))})
			continue
		}
		checks = append(checks, diagnosisCheck{nameID: "doctor.name.tool", nameArg: name, status: statusOK})
	}
	return checks
}

func doctorCheckPerms(root adapter.Root) []diagnosisCheck {
	var checks []diagnosisCheck
	centralPath := root.CentralConfigPath()
	if info, err := os.Stat(centralPath); err == nil && permLoose(info.Mode().Perm()) {
		checks = append(checks, diagnosisCheck{nameID: "doctor.name.perms", status: statusWarn, detail: i18n.New("doctor.detail.loosePerm", info.Mode().Perm(), centralPath, centralPath)})
	}
	if stateDir := root.StateDir(); filePermLoose(stateDir) {
		checks = append(checks, diagnosisCheck{nameID: "doctor.name.perms", status: statusWarn, detail: i18n.New("doctor.detail.statePermLoose", stateDir, stateDir)})
	}
	return checks
}

// permLoose reports whether group/other permission bits are set.
// Doctor and render share this single loose-permission judgment.
func permLoose(perm os.FileMode) bool {
	return perm&0o077 != 0
}

func filePermLoose(path string) bool {
	info, err := os.Stat(path)
	return err == nil && permLoose(info.Mode().Perm())
}
