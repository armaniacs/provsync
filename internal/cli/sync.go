// Package cli のうち、init / pull / push / sync を担う。
package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/store"
	"github.com/armaniacs/provsync/internal/syncer"
)

// ---- init ----

// pullResult は pullFromTool の結果。cmdPull / cmdInit / sync --from の
// from 経路で共有する。
type pullResult struct {
	Tool     string
	Change   plan.FileChange
	Central  *model.Config
	Pulled   map[string]model.Provider
	Warnings []*i18n.Message
}

// pullFromTool は「ツール設定を読み、provider を絞り込み、中央設定の変更計画を
// 作る」pull 側の前処理を集約する。cmdPull / cmdInit / sync --from の 3 経路で
// 共有する。新しい pull 前処理ステップはここに 1 箇所追加する。
// 警告は stderr へ出す（出力先・文言・順序は従来どおり）。
func (o *options) pullFromTool(root adapter.Root, toolName string) (pullResult, error) {
	a, err := adapter.Get(toolName, root)
	if err != nil {
		return pullResult{}, err
	}
	pulled, warnings, err := a.Pull()
	if err != nil {
		return pullResult{}, err
	}
	for _, w := range warnings {
		o.warnm(w)
	}
	pulled, err = syncer.FilterProviders(pulled, o.keys())
	if err != nil {
		return pullResult{}, err
	}
	change, cfg, err := buildCentralChange(root, a.Name(), pulled)
	if err != nil {
		return pullResult{}, err
	}
	return pullResult{Tool: a.Name(), Change: change, Central: cfg, Pulled: pulled, Warnings: warnings}, nil
}

// cmdInit は初回セットアップ用。pull と同じ Plan を再利用して中央設定を作る。
// 既存の中央設定は上書きしない。
func cmdInit(o *options, args []string) error {
	if len(args) > 1 {
		return o.usageErr("err.usage.init")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	central := root.CentralConfigPath()
	if _, err := os.Stat(central); err == nil {
		return i18n.New("err.init.alreadyInitialized", central)
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
			return i18n.New("err.init.noToolConfig", strings.Join(paths, "\n"))
		case 1:
			tool = found[0]
			o.msgf(o.out, "msg.init.detected", tool)
		default:
			o.msgf(o.out, "msg.init.multiple")
			for _, name := range found {
				fmt.Fprintf(o.out, "  %s\n", name)
			}
			o.msgf(o.out, "msg.init.specifyTool")
			return nil
		}
	}

	res, err := o.pullFromTool(root, tool)
	if err != nil {
		return err
	}
	if len(res.Pulled) == 0 {
		o.msgf(o.out, "msg.init.noProviders")
	}
	p := plan.Plan{Changes: []plan.FileChange{res.Change}}
	err = o.applyOrPreview("init "+res.Tool, p)
	if err != nil {
		return err
	}
	if len(res.Warnings) > 0 {
		o.msgf(o.errOut, "msg.init.secretHint")
	}
	if p.Changed() {
		if !o.write {
			o.msgf(o.out, "msg.init.next", res.Tool)
		} else {
			o.msgf(o.out, "msg.init.nextSteps")
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
		return o.usageErr("err.usage.pull")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	res, err := o.pullFromTool(root, args[0])
	if err != nil {
		return err
	}
	return o.applyOrPreview("pull "+res.Tool, plan.Plan{Changes: []plan.FileChange{res.Change}})
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

// toolChangeForPush は「中央設定を絞り込み、routes / aliases を適用し、ツール設定の
// 変更計画を作る」push 側の前処理を集約する。cmdPush と sync --to の 2 経路で共有する。
// 新しい push 前処理ステップはここに 1 箇所追加する。
func (o *options) toolChangeForPush(central *model.Config, a adapter.Adapter) (plan.FileChange, error) {
	managed, err := syncer.FilterProviders(central.Providers, o.keys())
	if err != nil {
		return plan.FileChange{}, err
	}
	managed = o.applyRoutes(managed, central)
	managed, err = o.applyAliases(managed, central, a)
	if err != nil {
		return plan.FileChange{}, err
	}
	return buildToolChange(a, managed)
}

func cmdPush(o *options, args []string) error {
	if len(args) != 1 {
		return o.usageErr("err.usage.push")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	central, err := store.Load(root.CentralConfigPath())
	if err != nil {
		return i18n.Wrap(err, "err.push.noCentral")
	}
	a, err := adapter.Get(args[0], root)
	if err != nil {
		return err
	}
	change, err := o.toolChangeForPush(central, a)
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
			o.warnf("warn.route.unused", alias)
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
		o.warnm(w)
	}
	if o.strict && len(warns) > 0 {
		return nil, i18n.New("err.alias.strict", len(warns))
	}
	return resolved, nil
}

// ---- sync ----

func cmdSync(o *options, args []string) error {
	if o.to == "" {
		return o.usageErr("err.usage.sync")
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
		res, err := o.pullFromTool(root, from)
		if err != nil {
			return plan.Plan{}, err
		}
		central = res.Central
		changes = append(changes, res.Change)
	} else {
		cfg, err := store.Load(root.CentralConfigPath())
		if err != nil {
			return plan.Plan{}, i18n.Wrap(err, "err.sync.noCentral")
		}
		central = cfg
	}

	toA, err := adapter.Get(to, root)
	if err != nil {
		return plan.Plan{}, err
	}
	change, err := o.toolChangeForPush(central, toA)
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
		return plan.FileChange{}, i18n.Wrap(err, "err.readTool.failed", a.Path())
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
// エラーは言語中立の Message で返し、描画(main の Localize)が言語を決める。
func checkReadable(path string) error {
	_, err := fsutil.ResolveSymlinkTarget(path)
	if err != nil {
		// The resolve error unwraps to ENOENT, so it must be matched first.
		var msg *i18n.Message
		if errors.As(err, &msg) && msg.ID == "err.symlink.resolve" {
			return err
		}
		if os.IsNotExist(err) {
			return i18n.New("err.tool.missing", path)
		}
		return err
	}
	return nil
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
