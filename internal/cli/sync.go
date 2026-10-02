// Package cli のうち、init / pull / push / sync を担う。
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/store"
	"github.com/armaniacs/provsync/internal/syncer"
)

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
