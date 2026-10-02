// Package cli はサブコマンドの解析と実行を担う。
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/backup"
	"github.com/armaniacs/provsync/internal/diff"
	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/lock"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/secret"
	"github.com/armaniacs/provsync/internal/store"
	"github.com/armaniacs/provsync/internal/syncer"
	"github.com/armaniacs/provsync/internal/version"
)

type options struct {
	out         io.Writer
	errOut      io.Writer
	write       bool
	noBackup    bool
	rootFlag    string
	providers   stringList
	from        string
	to          string
	list        bool
	prune       bool
	keep        int
	help        bool
	version     bool
	showSecrets bool
	jsonOut     bool
	exitCode    bool
	strict      bool
}

// stringList は --provider の繰り返し指定(カンマ区切り併用可)を蓄積する。
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*s = append(*s, part)
		}
	}
	return nil
}

// Run は provsync のエントリポイント。テスト可能なように writer を注入する。
func Run(args []string, out io.Writer) error {
	return RunWith(args, out, out)
}

// RunWith は stdout / stderr を分けて注入できるエントリポイント。
// 通常出力は out、警告とフラグ解析エラーは errOut へ出す。
func RunWith(args []string, out, errOut io.Writer) error {
	opts := &options{out: out, errOut: errOut, keep: backup.MaxOperations}
	fs := flag.NewFlagSet("provsync", flag.ContinueOnError)
	fs.SetOutput(errOut)
	registerFlags(fs, opts)
	if err := fs.Parse(reorder(fs, args)); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	pos := fs.Args()
	if opts.version {
		fmt.Fprintf(out, "provsync %s\n", version.String())
		return nil
	}
	if opts.help && len(pos) > 0 && helpTexts[pos[0]] != "" {
		fmt.Fprint(out, helpTexts[pos[0]])
		return nil
	}
	if opts.help || len(pos) == 0 {
		printUsage(opts)
		return nil
	}

	cmd, rest := pos[0], pos[1:]
	switch cmd {
	case "list":
		return cmdList(opts)
	case "status":
		return cmdStatus(opts, rest)
	case "init":
		return cmdInit(opts, rest)
	case "pull":
		return cmdPull(opts, rest)
	case "push":
		return cmdPush(opts, rest)
	case "sync":
		return cmdSync(opts)
	case "diff":
		return cmdDiff(opts, rest)
	case "undo":
		return cmdUndo(opts, rest)
	case "completion":
		return cmdCompletion(opts, rest)
	case "doctor":
		return cmdDoctor(opts)
	case "check":
		return cmdCheck(opts)
	case "version":
		fmt.Fprintf(opts.out, "provsync %s\n", version.String())
		return nil
	default:
		return usageErr("未知のコマンド %q です(--help を参照)", cmd)
	}
}

func registerFlags(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.rootFlag, "root", o.rootFlag, "パス解決の基準ディレクトリ(テスト用)")
	fs.BoolVar(&o.write, "write", o.write, "変更をファイルへ書き込む(既定はプレビュー)")
	fs.BoolVar(&o.noBackup, "no-backup", o.noBackup, "バックアップを記録しない(非推奨)")
	fs.Var(&o.providers, "provider", "対象 provider(カンマ区切り・繰り返し可)")
	fs.StringVar(&o.from, "from", o.from, "sync の取り込み元ツール")
	fs.StringVar(&o.to, "to", o.to, "sync の反映先ツール")
	fs.BoolVar(&o.list, "list", o.list, "undo の履歴を表示")
	fs.BoolVar(&o.prune, "prune", o.prune, "undo の履歴を掃除する")
	fs.IntVar(&o.keep, "keep", o.keep, "残す履歴数(--prune 用)")
	fs.BoolVar(&o.jsonOut, "json", o.jsonOut, "JSON で出力する(list / status / diff)")
	fs.BoolVar(&o.exitCode, "exit-code", o.exitCode, "差分があるとき終了コード 3 で終了する(status)")
	fs.BoolVar(&o.strict, "strict", o.strict, "エイリアス未定義のモデル名があるときエラーにする(push)")
	fs.BoolVar(&o.version, "version", o.version, "バージョンを表示")
	fs.BoolVar(&o.showSecrets, "show-secrets", o.showSecrets, "diff の出力で秘密の値をそのまま表示する(非推奨)")
	fs.BoolVar(&o.help, "help", o.help, "ヘルプを表示")
	fs.BoolVar(&o.help, "h", o.help, "ヘルプを表示")
}

// reorder はフラグと位置引数を分離し、フラグを前に寄せる。
// これにより `push opencode --write` のような後置フラグも受理できる。
func reorder(fs *flag.FlagSet, args []string) []string {
	boolFlag := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			boolFlag[f.Name] = true
		}
	})
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" {
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				flags = append(flags, a)
				continue
			}
			flags = append(flags, a)
			if !boolFlag[name] && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	return append(flags, positional...)
}

func (o *options) root() (adapter.Root, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return adapter.Root{}, err
	}
	return adapter.ResolveRoot(home, o.rootFlag), nil
}

func (o *options) keys() []string {
	return []string(o.providers)
}

// printUsage は使い方と、読み書きする設定ファイルの場所を出す。
// HOME が解決できない場合でも使い方本文は出し、設定ファイル節だけ省略する。
func printUsage(o *options) {
	fmt.Fprintln(o.out, "provsync "+version.String())
	fmt.Fprintln(o.out, `使い方: provsync <command> [flags]

コマンド:
  list                     対応ツールと設定パスを表示
  status [tool...]         ツールと中央設定の同期状態を表示
  init [tool]              初回セットアップ(中央設定を作る)
  pull <tool>              ツール設定を中央設定へ取り込む
  push <tool>              中央設定をツール設定へ反映する
  sync --from <a> --to <b> a を取り込み b へ反映する(--from 省略可)
  diff <from> <to>         from を to に適用した場合の差分を表示
  undo [id]                直前または指定操作を復元する(--list で履歴)
  doctor                   環境を診断する(通信しない)
  check                    各 provider の API 到達可否を確認する(明示実行のみ)

共通フラグ:
  --write          変更を書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)
  --no-backup      バックアップを記録しない
  --root <dir>     パス解決の基準を差し替える(テスト用)`)

	root, err := o.root()
	if err != nil {
		return
	}
	fmt.Fprintln(o.out, "\n設定ファイル:")
	central := root.CentralConfigPath()
	if _, err := os.Stat(central); err == nil {
		fmt.Fprintf(o.out, "  中央設定: %s\n", central)
	} else {
		fmt.Fprintf(o.out, "  中央設定: %s (未作成)\n", central)
		fmt.Fprintln(o.out, "    provsync init <tool> --write で作成します")
	}
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			continue
		}
		if _, err := os.Stat(a.Path()); err == nil {
			fmt.Fprintf(o.out, "  %-9s %s\n", name, a.Path())
		} else {
			fmt.Fprintf(o.out, "  %-9s %s (未作成)\n", name, a.Path())
		}
	}
	fmt.Fprintf(o.out, "  バックアップ: %s\n", root.StateDir())
}

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

// encodeJSON は v を 2 スペースインデントの JSON で出力する。
func encodeJSON(out io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
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
			for _, line := range driftLines(a.Project(central.Providers), providers) {
				if line == "差分なし" {
					continue
				}
				ts.Drift = append(ts.Drift, line)
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

// driftLines は projected(ツール可視の形へ写した中央設定)と tool の
// provider 集合を比較し、status 表示用の行を返す。
func driftLines(projected, tool map[string]model.Provider) []string {
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

	var lines []string
	for _, k := range keys {
		c, inCentral := projected[k]
		t, inTool := tool[k]
		switch {
		case inCentral && !inTool:
			lines = append(lines, fmt.Sprintf("%s: ツールに無い", k))
		case !inCentral && inTool:
			lines = append(lines, fmt.Sprintf("%s: 中央に無い", k))
		case len(plan.ChangedFields(c, t)) > 0:
			lines = append(lines, fmt.Sprintf("%s: 差分あり", k))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "差分なし")
	}
	return lines
}

// ---- init ----

// cmdInit は初回セットアップ用。pull と同じ Plan を再利用して中央設定を作る。
// 既存の中央設定は上書きしない。
func cmdInit(o *options, args []string) error {
	if len(args) > 1 {
		return usageErr("使い方: provsync init [tool]")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	central := root.CentralConfigPath()
	if _, err := os.Stat(central); err == nil {
		return fmt.Errorf("すでに初期化されています: %s\n日常の更新には provsync pull <tool> を使ってください", central)
	}

	tool := ""
	if len(args) == 1 {
		tool = args[0]
	} else {
		found := detectTools(root)
		switch len(found) {
		case 0:
			var paths []string
			for _, name := range adapter.Names() {
				a, err := adapter.Get(name, root)
				if err != nil {
					return err
				}
				paths = append(paths, a.Path())
			}
			return fmt.Errorf("対応ツールの設定ファイルが見つかりません。探した場所:\n%s\nツールを先に設定してから再実行してください", strings.Join(paths, "\n"))
		case 1:
			tool = found[0]
			fmt.Fprintf(o.out, "検出したツール: %s\n", tool)
		default:
			fmt.Fprintln(o.out, "複数のツール設定が見つかりました:")
			for _, name := range found {
				fmt.Fprintf(o.out, "  %s\n", name)
			}
			fmt.Fprintln(o.out, "provsync init <tool> でツールを指定してください")
			return nil
		}
	}

	a, err := adapter.Get(tool, root)
	if err != nil {
		return err
	}
	pulled, warnings, err := a.Pull()
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(o.errOut, "警告: %s\n", w)
	}
	pulled, err = syncer.FilterProviders(pulled, o.keys())
	if err != nil {
		return err
	}

	change, _, err := buildCentralChange(root, a.Name(), pulled)
	if err != nil {
		return err
	}
	if len(pulled) == 0 {
		fmt.Fprintln(o.out, "取り込める provider がありません")
	}
	p := plan.Plan{Changes: []plan.FileChange{change}}
	err = o.applyOrPreview("init "+a.Name(), p)
	if err != nil {
		return err
	}
	if len(warnings) > 0 {
		fmt.Fprintln(o.errOut, "秘密は中央設定に保存されません。環境変数名を中央設定の apiKeyEnv に設定してください")
	}
	if p.Changed() {
		if !o.write {
			fmt.Fprintf(o.out, "次に: provsync init %s --write で中央設定を作成します\n", a.Name())
		} else {
			fmt.Fprintln(o.out, "次の手順:")
			fmt.Fprintln(o.out, "  1. provsync status              同期状態を確認する")
			fmt.Fprintln(o.out, "  2. provsync push <他のツール>    他のツールへ反映する(まずプレビュー)")
			fmt.Fprintln(o.out, "  3. 問題があれば provsync undo で元に戻せます")
		}
	}
	return nil
}

// detectTools は設定ファイルが存在するツール名を返す。
func detectTools(root adapter.Root) []string {
	var found []string
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			continue
		}
		if _, err := os.Stat(a.Path()); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// ---- pull ----

func cmdPull(o *options, args []string) error {
	if len(args) != 1 {
		return usageErr("使い方: provsync pull <tool>")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	a, err := adapter.Get(args[0], root)
	if err != nil {
		return err
	}
	pulled, warnings, err := a.Pull()
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(o.errOut, "警告: %s\n", w)
	}
	pulled, err = syncer.FilterProviders(pulled, o.keys())
	if err != nil {
		return err
	}

	change, _, err := buildCentralChange(root, a.Name(), pulled)
	if err != nil {
		return err
	}
	return o.applyOrPreview("pull "+a.Name(), plan.Plan{Changes: []plan.FileChange{change}})
}

// buildCentralChange は tool から取り込んだ pulled を中央設定へマージした
// FileChange と、マージ後のカノニカル設定を返す。tool 以外の名前空間と
// version は既存の中央設定から引き継ぐ。
func buildCentralChange(root adapter.Root, tool string, pulled map[string]model.Provider) (plan.FileChange, *model.Config, error) {
	centralPath := root.CentralConfigPath()
	base, err := loadCentralOrNew(centralPath)
	if err != nil {
		return plan.FileChange{}, nil, err
	}
	before, err := readFileOptional(centralPath)
	if err != nil {
		return plan.FileChange{}, nil, err
	}
	merged := syncer.MergeToolProviders(base.Providers, pulled, tool)
	version := base.Version
	if version == 0 {
		version = model.Version
	}
	// aliases / routes はユーザー定義の正。pull のたびに消えないよう引き継ぐ。
	cfg := &model.Config{Version: version, Providers: merged, Aliases: base.Aliases, Routes: base.Routes}
	after, err := store.Marshal(cfg)
	if err != nil {
		return plan.FileChange{}, nil, err
	}
	change := plan.FileChange{
		Tool:     "central",
		Path:     centralPath,
		Before:   before,
		After:    after,
		Semantic: plan.ProvidersDiff(pulled, base.Providers),
	}
	return change, cfg, nil
}

// ---- push ----

func cmdPush(o *options, args []string) error {
	if len(args) != 1 {
		return usageErr("使い方: provsync push <tool>")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	central, err := store.Load(root.CentralConfigPath())
	if err != nil {
		return fmt.Errorf("中央設定がありません。先に pull / sync を実行してください: %w", err)
	}
	a, err := adapter.Get(args[0], root)
	if err != nil {
		return err
	}
	managed, err := syncer.FilterProviders(central.Providers, o.keys())
	if err != nil {
		return err
	}
	managed = o.applyRoutes(managed, central)
	managed, err = o.applyAliases(managed, central, a)
	if err != nil {
		return err
	}
	change, err := buildToolChange(a, managed)
	if err != nil {
		return err
	}
	return o.applyOrPreview("push "+a.Name(), plan.Plan{Changes: []plan.FileChange{change}})
}

// applyRoutes は push の描画対象から、routes で選ばれなかった経路の provider を
// 除く。中央設定は変えない。どの経路も選べなかったときは警告し、provider は
// 変更せず残す。判定は環境変数の有無のみで、通信しない。
func (o *options) applyRoutes(managed map[string]model.Provider, central *model.Config) map[string]model.Provider {
	if len(central.Routes) == 0 {
		return managed
	}
	lookup := func(name string) bool {
		_, ok := os.LookupEnv(name)
		return ok
	}
	aliases := make([]string, 0, len(central.Routes))
	for a := range central.Routes {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		key, ok := syncer.SelectRoute(central.Routes[alias], managed, lookup)
		if !ok {
			fmt.Fprintf(o.errOut, "警告: エイリアス %q の経路で使える provider がありません(apiKeyEnv が未設定)。変更せず残します\n", alias)
			continue
		}
		for _, cand := range central.Routes[alias] {
			if cand != key {
				delete(managed, cand)
			}
		}
	}
	return managed
}

// applyAliases は push の描画前に managed のモデル名をツール向け ID に変換する。
// pull には適用しない(ID を書き換えないため)。未定義のエイリアスは警告して
// 素通しし、--strict 指定時はエラーにする。
func (o *options) applyAliases(managed map[string]model.Provider, central *model.Config, a adapter.Adapter) (map[string]model.Provider, error) {
	resolved, warns := syncer.ResolveAliases(managed, central.Aliases, a.Name())
	for _, w := range warns {
		fmt.Fprintf(o.errOut, "警告: %s\n", w)
	}
	if o.strict && len(warns) > 0 {
		return nil, fmt.Errorf("エイリアス未定義のモデル名があります (--strict): %d 件", len(warns))
	}
	return resolved, nil
}

// ---- sync ----

func cmdSync(o *options) error {
	if o.to == "" {
		return usageErr("使い方: provsync sync --from <a> --to <b>(--from は省略可)")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	p, err := buildSyncPlan(o, root, o.from, o.to)
	if err != nil {
		return err
	}
	label := "sync"
	if o.from != "" {
		label = "sync " + o.from + " -> " + o.to
	}
	return o.applyOrPreview(label, p)
}

func buildSyncPlan(o *options, root adapter.Root, from, to string) (plan.Plan, error) {
	var changes []plan.FileChange

	var central *model.Config
	if from != "" {
		fromA, err := adapter.Get(from, root)
		if err != nil {
			return plan.Plan{}, err
		}
		pulled, warnings, err := fromA.Pull()
		if err != nil {
			return plan.Plan{}, err
		}
		for _, w := range warnings {
			fmt.Fprintf(o.errOut, "警告: %s\n", w)
		}
		pulled, err = syncer.FilterProviders(pulled, o.keys())
		if err != nil {
			return plan.Plan{}, err
		}
		change, cfg, err := buildCentralChange(root, fromA.Name(), pulled)
		if err != nil {
			return plan.Plan{}, err
		}
		central = cfg
		changes = append(changes, change)
	} else {
		cfg, err := store.Load(root.CentralConfigPath())
		if err != nil {
			return plan.Plan{}, fmt.Errorf("中央設定がありません。--from を指定するか先に pull してください: %w", err)
		}
		central = cfg
	}

	toA, err := adapter.Get(to, root)
	if err != nil {
		return plan.Plan{}, err
	}
	managed, err := syncer.FilterProviders(central.Providers, o.keys())
	if err != nil {
		return plan.Plan{}, err
	}
	managed = o.applyRoutes(managed, central)
	managed, err = o.applyAliases(managed, central, toA)
	if err != nil {
		return plan.Plan{}, err
	}
	change, err := buildToolChange(toA, managed)
	if err != nil {
		return plan.Plan{}, err
	}
	changes = append(changes, change)
	return plan.Plan{Changes: changes}, nil
}

func buildToolChange(a adapter.Adapter, managed map[string]model.Provider) (plan.FileChange, error) {
	if err := checkReadable(a.Path()); err != nil {
		return plan.FileChange{}, err
	}
	before, err := os.ReadFile(a.Path())
	if err != nil {
		return plan.FileChange{}, fmt.Errorf("ツール設定を読めません (%s): %w", a.Path(), err)
	}
	after, err := a.Push(managed)
	if err != nil {
		return plan.FileChange{}, err
	}
	current, _, err := a.Pull()
	if err != nil {
		return plan.FileChange{}, err
	}
	// 意味差分は「このツールへ push した場合に描画される内容」と比較する。
	// apiKeyEnv 等、ツールが描画しないフィールドは誤検知の原因になるため
	// Project でツール可視の形へ写してから比べる。
	return plan.FileChange{
		Tool:     a.Name(),
		Path:     a.Path(),
		Before:   before,
		After:    after,
		Semantic: plan.ProvidersDiff(a.Project(managed), current),
	}, nil
}

// checkReadable は壊れたシンボリックリンクを明確なエラーにする。
// os.ReadFile だけではリンク切れと欠落を区別できない。
func checkReadable(path string) error {
	li, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ツール設定がありません: %s", path)
		}
		return err
	}
	if li.Mode()&os.ModeSymlink != 0 {
		if _, err := filepath.EvalSymlinks(path); err != nil {
			return fmt.Errorf("シンボリックリンクのリンク先を解決できません (%s): %w", path, err)
		}
	}
	return nil
}

// ---- diff ----

func cmdDiff(o *options, args []string) error {
	if len(args) != 2 {
		return usageErr("使い方: provsync diff <from> <to>")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	p, err := buildSyncPlan(o, root, args[0], args[1])
	if err != nil {
		return err
	}
	if o.jsonOut {
		rep := diffReport{SchemaVersion: 1, Changes: []diffChange{}}
		for _, c := range p.Changes {
			d := diff.Unified(c.Path, c.Before, c.After, 3)
			if !o.showSecrets {
				d = secret.MaskLines(d)
			}
			rep.Changes = append(rep.Changes, diffChange{
				Tool:     c.Tool,
				Path:     c.Path,
				Semantic: c.Semantic,
				Diff:     d,
			})
		}
		return encodeJSON(o.out, rep)
	}
	for _, c := range p.Changes {
		fmt.Fprintf(o.out, "%s: %s\n", c.Tool, c.Path)
		renderSemantic(o.out, c.Semantic)
		d := diff.Unified(c.Path, c.Before, c.After, 3)
		if d == "" {
			fmt.Fprintln(o.out, "  変更なし")
			continue
		}
		if !o.showSecrets {
			d = secret.MaskLines(d)
		} else {
			fmt.Fprintln(o.errOut, "警告: --show-secrets により秘密の値をそのまま表示しています")
		}
		fmt.Fprint(o.out, d)
	}
	return nil
}

// diffReport は diff の出力。--json で使う。
type diffReport struct {
	SchemaVersion int          `json:"schemaVersion"`
	Changes       []diffChange `json:"changes"`
}

type diffChange struct {
	Tool     string                `json:"tool"`
	Path     string                `json:"path"`
	Semantic []plan.ProviderChange `json:"semantic"`
	Diff     string                `json:"diff"`
}

// ---- undo ----

func cmdUndo(o *options, args []string) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	st := backup.New(root.StateDir())
	max, err := keepFromEnv()
	if err != nil {
		return err
	}
	st.SetMax(max)

	if o.prune {
		removed, err := st.Prune(o.keep)
		if err != nil {
			return err
		}
		fmt.Fprintf(o.out, "削除: %d 件\n", removed)
		return nil
	}

	if o.list {
		ops, err := st.List()
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			fmt.Fprintln(o.out, "履歴はありません")
			return nil
		}
		for _, op := range ops {
			status := ""
			if op.NoBackup {
				status = " [バックアップなし]"
			} else if op.Undone {
				status = " [undo 済み]"
			}
			fmt.Fprintf(o.out, "%s  %s  %s%s\n", op.ID, op.StartedAt.Format("2006-01-02 15:04:05"), op.Command, status)
			for _, f := range op.Files {
				fmt.Fprintf(o.out, "    %s\n", f.Path)
			}
		}
		return nil
	}

	id := ""
	if len(args) > 0 {
		id = args[0]
	}
	op, err := st.Find(id)
	if err != nil {
		return err
	}
	if ops, err := st.List(); err == nil && len(ops) > 0 {
		newest := ops[0]
		if newest.NoBackup && newest.ID != op.ID {
			fmt.Fprintln(o.errOut, "警告: 直近の書き込みはバックアップなしで行われたため、この undo はそれより前の状態に戻します")
		}
	}
	// 復元(索引の読み → 更新 → 保存とファイル復元)の区間だけ排他する。
	release, err := lock.Acquire(root.StateDir(), 10*time.Second)
	if err != nil {
		return err
	}
	defer release()
	undoOp, err := st.Restore(op)
	if err != nil {
		return err
	}
	fmt.Fprintf(o.out, "復元しました: %s (%s)\n", op.ID, op.Command)
	fmt.Fprintf(o.out, "やり直し: provsync undo %s\n", undoOp.ID)
	for _, f := range op.Files {
		if f.Existed {
			fmt.Fprintf(o.out, "  復元: %s\n", f.Path)
		} else {
			fmt.Fprintf(o.out, "  削除: %s\n", f.Path)
		}
	}
	return nil
}

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

// ---- check ----

// checkTimeout は疎通確認 1 回あたりのタイムアウト。テストで上書きする。
var checkTimeout = 5 * time.Second

// cmdCheck は中央設定の各 provider の API 到達可否を確認する。
// 通信するのはこのコマンドだけ。秘密の値は出力しない。
func cmdCheck(o *options) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	cfg, err := store.Load(root.CentralConfigPath())
	if err != nil {
		return fmt.Errorf("中央設定がありません。先に pull / init を実行してください: %w", err)
	}
	keys := make([]string, 0, len(cfg.Providers))
	for k := range cfg.Providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	filtered, err := syncer.FilterProviders(cfg.Providers, o.keys())
	if err != nil {
		return err
	}
	if o.keys() != nil {
		keys = make([]string, 0, len(filtered))
		for k := range filtered {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	for _, k := range keys {
		p := cfg.Providers[k]
		switch {
		case p.APIKeyEnv == "":
			fmt.Fprintf(o.out, "[スキップ] %s: apiKeyEnv が未設定\n", k)
			continue
		}
		if _, ok := os.LookupEnv(p.APIKeyEnv); !ok {
			// 変数の値は読んでも出力しない。
			fmt.Fprintf(o.out, "[スキップ] %s: 環境変数 %s が未設定\n", k, p.APIKeyEnv)
			continue
		}
		if p.BaseURL == "" {
			fmt.Fprintf(o.out, "[スキップ] %s: baseURL が未設定\n", k)
			continue
		}
		checkProvider(o, k, p)
	}
	return nil
}

// checkProvider は 1 provider に GET <BaseURL>/models を送り結果を分類して出す。
func checkProvider(o *options, key string, p model.Provider) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	url := strings.TrimSuffix(p.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(o.out, "[応答異常] %s: リクエストを構築できません\n", key)
		return
	}
	token := os.Getenv(p.APIKeyEnv)
	req.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// エラー文字列には URL やヘッダが入らないよう、分類済みの短い文言にする。
		fmt.Fprintf(o.out, "[到達不可] %s\n", key)
		return
	}
	defer resp.Body.Close()
	ms := time.Since(start).Milliseconds()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		fmt.Fprintf(o.out, "[OK] %s (%dms)\n", key, ms)
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		fmt.Fprintf(o.out, "[認証失敗] %s\n", key)
	default:
		fmt.Fprintf(o.out, "[応答異常] %s: HTTP %d\n", key, resp.StatusCode)
	}
}

// helpTexts はサブコマンド別のヘルプ。`provsync <cmd> --help` で出す。
var helpTexts = map[string]string{
	"list": `list - 対応ツールと設定パスを表示

用途: 各ツールの設定ファイルと中央設定のパス、provider 数を表示する。

使い方: provsync list

例:
  provsync list
`,
	"status": `status - ツールと中央設定の同期状態を表示

用途: ツール設定と中央設定の provider 差分( drift )を表示する。

使い方: provsync status [tool...]

引数: tool を省略すると全対応ツールを表示する。

関連フラグ:
  --json    JSON で出力する

例:
  provsync status
  provsync status kilocode
`,
	"init": `init - 初回セットアップ(中央設定を作る)

用途: ツール設定から中央設定を作成する。初回専用で、既存の中央設定は上書きしない。

使い方: provsync init [tool]

引数: tool を省略すると、設定ファイルが存在するツールを検出する。
      候補が複数ある場合は一覧を表示するので指定する。

関連フラグ:
  --write   中央設定を作成する(既定はプレビュー)

例:
  provsync init kilocode
  provsync init kilocode --write
`,
	"pull": `pull - ツール設定を中央設定へ取り込む

用途: ツール設定の provider エントリを中央設定へマージする。

使い方: provsync pull <tool>

引数: tool は kilocode(別名 kilo) / opencode。

関連フラグ:
  --write          中央設定へ書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync pull kilocode
  provsync pull kilocode --write
`,
	"push": `push - 中央設定をツール設定へ反映する

用途: 中央設定の provider エントリをツール設定へマージする。

使い方: provsync push <tool>

引数: tool は kilocode(別名 kilo) / opencode。

関連フラグ:
  --write          ツール設定へ書き込む(既定はプレビュー)
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync push opencode
  provsync push opencode --write
`,
	"sync": `sync - 取り込みと反映を一度に行う

用途: --from のツール設定を中央設定へ取り込み、--to のツール設定へ反映する。

使い方: provsync sync --from <a> --to <b>

引数: --from を省略すると中央設定をそのまま使う。

関連フラグ:
  --from <tool>   取り込み元ツール(省略可)
  --to <tool>     反映先ツール(必須)
  --write         ファイルへ書き込む(既定はプレビュー)

例:
  provsync sync --from kilocode --to opencode --write
`,
	"diff": `diff - 適用した場合の差分を表示

用途: from を to に適用した場合の意味差分と統合 diff を表示する。

使い方: provsync diff <from> <to>

引数: from / to はツール名か central。

関連フラグ:
  --show-secrets   秘密の値をそのまま表示する(非推奨)

例:
  provsync diff kilocode opencode
`,
	"undo": `undo - 直前または指定操作を復元する

用途: 書き込み操作をバックアップから復元する。--write は不要で直接適用する。

使い方: provsync undo [id]

引数: id を省略すると直前の書き込みを復元する。undo 自体を undo できる(redo)。

関連フラグ:
  --list   履歴を表示する
  --prune / --keep <n>   履歴を掃除する

例:
  provsync undo
  provsync undo --list
  provsync undo 20261002T093045-8c2d
`,
	"doctor": `doctor - 環境を診断する

用途: 設定の存在・構文・apiKeyEnv の環境変数・権限を一覧で診断する。

使い方: provsync doctor

通信せず、ファイルも書かない。NG があるときは終了コード 1 で終わる。

例:
  provsync doctor
`,
	"check": `check - API の疎通確認

用途: 中央設定の各 provider について API への到達可否を確認する。

使い方: provsync check

通信するのはこのコマンドだけ。他のコマンドから呼ばない。
タイムアウトは 1 provider あたり 5 秒。秘密の値は出力しない。

関連フラグ:
  --provider <p>   対象 provider を限定(カンマ区切り)

例:
  provsync check
`,
	"completion": `completion - シェル補完スクリプトを出力

用途: bash / zsh / fish 用の補完スクリプトを標準出力へ出す。

使い方: provsync completion <shell>

引数: shell は bash / zsh / fish。

例:
  # zsh
  provsync completion zsh > "${fpath[1]}/_provsync"

  # bash
  source <(provsync completion bash)
`,
	"version": `version - バージョンを表示

用途: バイナリのバージョンを 1 行で表示する。

使い方: provsync version

例:
  provsync version
  provsync --version
`,
}

// cmdCompletion はシェル補完スクリプトを出力する。
func cmdCompletion(o *options, args []string) error {
	if len(args) != 1 {
		return usageErr("使い方: provsync completion <bash|zsh|fish>")
	}
	shell := args[0]
	tools := adapter.Names()
	switch shell {
	case "bash":
		fmt.Fprintf(o.out, `_provsync() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
  else
    COMPREPLY=( $(compgen -W "%s --write --provider --no-backup --show-secrets --json --exit-code" -- "$cur") )
  fi
}
complete -F _provsync provsync
`, strings.Join(commands, " "), strings.Join(tools, " "))
	case "zsh":
		fmt.Fprintf(o.out, "#compdef provsync\n\n_provsync() {\n  _arguments '1: :(%s)' '*: :(%s)'\n}\ncompdef _provsync provsync\n", strings.Join(commands, " "), strings.Join(tools, " "))
	case "fish":
		fmt.Fprintf(o.out, "complete -c provsync -n '__fish_use_subcommand' -a '%s'\n", strings.Join(commands, " "))
		fmt.Fprintf(o.out, "complete -c provsync -n '__fish_seen_subcommand_from pull push' -a '%s'\n", strings.Join(tools, " "))
	default:
		return usageErr("未知のシェル %q です(有効: bash / zsh / fish)", shell)
	}
	return nil
}

var commands = []string{"list", "status", "init", "pull", "push", "sync", "diff", "undo", "doctor", "check", "completion", "version"}

// ---- 共通 ----

// keepFromEnv はバックアップ保持数を環境変数 PROVSYNC_KEEP から読む。
// 未設定は backup.Store の既定値を使う。1 未満・非数はエラー。
func keepFromEnv() (int, error) {
	v := os.Getenv("PROVSYNC_KEEP")
	if v == "" {
		return backup.MaxOperations, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("PROVSYNC_KEEP は 1 以上の整数で指定してください (値: %s)", v)
	}
	return n, nil
}

// warnLoosePerm は path が他ユーザーから読める権限なら chmod を案内する。
func warnLoosePerm(out io.Writer, path string, dir bool) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.Mode().Perm()&0o077 == 0 {
		return
	}
	perm := info.Mode().Perm()
	suggest := "600"
	if dir {
		suggest = "700"
	}
	fmt.Fprintf(out, "警告: 権限が緩い (%04o): %s  chmod %s %s\n", perm, path, suggest, path)
}

func (o *options) applyOrPreview(label string, p plan.Plan) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	renderPreview(o.out, p)
	if !p.Changed() {
		fmt.Fprintln(o.out, "変更はありません")
		return nil
	}
	if !o.write {
		fmt.Fprintln(o.out, "(プレビューのみ; 適用するには --write)")
		return nil
	}
	changed := make([]plan.FileChange, 0, len(p.Changes))
	for _, c := range p.Changes {
		if !bytes.Equal(c.Before, c.After) {
			changed = append(changed, c)
		}
	}
	// 書き込みとバックアップ記録の区間だけ排他する。読み取り専用の経路は待たせない。
	release, err := lock.Acquire(root.StateDir(), 10*time.Second)
	if err != nil {
		return err
	}
	defer release()
	if !o.noBackup {
		paths := make([]string, 0, len(changed))
		for _, c := range changed {
			paths = append(paths, c.Path)
		}
		st := backup.New(root.StateDir())
		max, err := keepFromEnv()
		if err != nil {
			return err
		}
		st.SetMax(max)
		op, err := st.Record(label, paths)
		if err != nil {
			return fmt.Errorf("バックアップに失敗しました: %w", err)
		}
		fmt.Fprintf(o.out, "バックアップ: %s\n", op.ID)
	} else if _, err := backup.New(root.StateDir()).RecordMarker(label); err != nil {
		return fmt.Errorf("履歴の記録に失敗しました: %w", err)
	}
	written := make([]string, 0, len(changed))
	for _, c := range changed {
		if err := fsutil.WriteFileAtomic(c.Path, c.After); err != nil {
			if len(written) > 0 {
				fmt.Fprintf(o.errOut, "警告: この操作で既に書き込み済みのファイル: %s\n", strings.Join(written, ", "))
				if !o.noBackup {
					fmt.Fprintln(o.out, "ヒント: provsync undo でこの操作をまとめて復元できます")
				}
			}
			return fmt.Errorf("書き込みに失敗しました (%s): %w", c.Path, err)
		}
		written = append(written, c.Path)
		fmt.Fprintf(o.out, "書き込み: %s\n", c.Path)
	}
	return nil
}

func renderPreview(out io.Writer, p plan.Plan) {
	for _, c := range p.Changes {
		fmt.Fprintf(out, "%s: %s\n", c.Tool, c.Path)
		renderSemantic(out, c.Semantic)
	}
}

func renderSemantic(out io.Writer, changes []plan.ProviderChange) {
	shown := 0
	for _, ch := range changes {
		if ch.Op == "unchanged" {
			continue
		}
		shown++
		if len(ch.Fields) > 0 {
			fmt.Fprintf(out, "  %s: %s (%s)\n", ch.Key, opLabel(ch.Op), strings.Join(ch.Fields, ", "))
		} else {
			fmt.Fprintf(out, "  %s: %s\n", ch.Key, opLabel(ch.Op))
		}
	}
	if shown == 0 {
		fmt.Fprintln(out, "  変更なし")
	}
}

func opLabel(op string) string {
	switch op {
	case "added":
		return "追加"
	case "updated":
		return "更新"
	default:
		return "変更なし"
	}
}

// loadCentralOrNew は中央設定を読み込む。ファイルが無ければ新規設定を返し、
// 存在するのに壊れている場合はエラーを返す。
func loadCentralOrNew(path string) (*model.Config, error) {
	cfg, err := store.Load(path)
	if err != nil {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return model.NewConfig(), nil
		}
		return nil, err
	}
	return cfg, nil
}

func readFileOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}
