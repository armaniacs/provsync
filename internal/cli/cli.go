// Package cli はサブコマンドの解析と実行を担う。
package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/backup"
	"github.com/armaniacs/provsync/internal/diff"
	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/secret"
	"github.com/armaniacs/provsync/internal/store"
	"github.com/armaniacs/provsync/internal/syncer"
	"github.com/armaniacs/provsync/internal/version"
)

type options struct {
	out         io.Writer
	write       bool
	noBackup    bool
	rootFlag    string
	providers   stringList
	from        string
	to          string
	list        bool
	help        bool
	version     bool
	showSecrets bool
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
	opts := &options{out: out}
	fs := flag.NewFlagSet("provsync", flag.ContinueOnError)
	fs.SetOutput(out)
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
	if opts.help || len(pos) == 0 {
		printUsage(opts)
		return nil
	}

	cmd, rest := pos[0], pos[1:]
	switch cmd {
	case "list":
		return cmdList(opts)
	case "init":
		return cmdInit(opts, rest)
	case "version":
		fmt.Fprintf(opts.out, "provsync %s\n", version.String())
		return nil
	case "status":
		return cmdStatus(opts, rest)
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
	default:
		return fmt.Errorf("未知のコマンド %q です(--help を参照)", cmd)
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
	central := root.CentralConfigPath()
	if _, err := os.Stat(central); err == nil {
		cfg, err := store.Load(central)
		if err != nil {
			return err
		}
		fmt.Fprintf(o.out, "中央設定: %s (%d providers)\n", central, len(cfg.Providers))
	} else {
		fmt.Fprintf(o.out, "中央設定: %s (未作成)\n", central)
	}
	for _, name := range adapter.Names() {
		a, err := adapter.Get(name, root)
		if err != nil {
			return err
		}
		path := a.Path()
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintf(o.out, "%-9s %s (未作成)\n", name, path)
			continue
		}
		providers, _, err := a.Pull()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.out, "%-9s %s (%d providers)\n", name, path, len(providers))
	}
	return nil
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

	var central *model.Config
	if cfg, err := store.Load(root.CentralConfigPath()); err == nil {
		central = cfg
		fmt.Fprintf(o.out, "中央設定: %s (%d providers)\n", root.CentralConfigPath(), len(cfg.Providers))
	} else if _, statErr := os.Stat(root.CentralConfigPath()); os.IsNotExist(statErr) {
		fmt.Fprintf(o.out, "中央設定: %s (未作成)\n", root.CentralConfigPath())
	} else {
		return err
	}

	for _, name := range tools {
		a, err := adapter.Get(name, root)
		if err != nil {
			return err
		}
		path := a.Path()
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintf(o.out, "%-9s %s (未作成)\n", name, path)
			continue
		}
		providers, warnings, err := a.Pull()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.out, "%-9s %s (%d providers)\n", name, path, len(providers))
		for _, w := range warnings {
			fmt.Fprintf(o.out, "  警告: %s\n", w)
		}
		if central == nil {
			continue
		}
		for _, line := range driftLines(a.Project(central.Providers), providers) {
			fmt.Fprintf(o.out, "  %s\n", line)
		}
	}
	return nil
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
		return fmt.Errorf("使い方: provsync init [tool]")
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
		fmt.Fprintf(o.out, "警告: %s\n", w)
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
		fmt.Fprintln(o.out, "秘密は中央設定に保存されません。環境変数名を中央設定の apiKeyEnv に設定してください")
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
		return fmt.Errorf("使い方: provsync pull <tool>")
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
		fmt.Fprintf(o.out, "警告: %s\n", w)
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
	cfg := &model.Config{Version: version, Providers: merged}
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
		return fmt.Errorf("使い方: provsync push <tool>")
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
	change, err := buildToolChange(a, managed)
	if err != nil {
		return err
	}
	return o.applyOrPreview("push "+a.Name(), plan.Plan{Changes: []plan.FileChange{change}})
}

// ---- sync ----

func cmdSync(o *options) error {
	if o.to == "" {
		return fmt.Errorf("使い方: provsync sync --from <a> --to <b>(--from は省略可)")
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
			fmt.Fprintf(o.out, "警告: %s\n", w)
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
	change, err := buildToolChange(toA, managed)
	if err != nil {
		return plan.Plan{}, err
	}
	changes = append(changes, change)
	return plan.Plan{Changes: changes}, nil
}

func buildToolChange(a adapter.Adapter, managed map[string]model.Provider) (plan.FileChange, error) {
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

// ---- diff ----

func cmdDiff(o *options, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("使い方: provsync diff <from> <to>")
	}
	root, err := o.root()
	if err != nil {
		return err
	}
	p, err := buildSyncPlan(o, root, args[0], args[1])
	if err != nil {
		return err
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
			fmt.Fprintln(o.out, "警告: --show-secrets により秘密の値をそのまま表示しています")
		}
		fmt.Fprint(o.out, d)
	}
	return nil
}

// ---- undo ----

func cmdUndo(o *options, args []string) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	st := backup.New(root.StateDir())

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
			fmt.Fprintln(o.out, "警告: 直近の書き込みはバックアップなしで行われたため、この undo はそれより前の状態に戻します")
		}
	}
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

// ---- 共通 ----

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
	if !o.noBackup {
		paths := make([]string, 0, len(changed))
		for _, c := range changed {
			paths = append(paths, c.Path)
		}
		op, err := backup.New(root.StateDir()).Record(label, paths)
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
				fmt.Fprintf(o.out, "警告: この操作で既に書き込み済みのファイル: %s\n", strings.Join(written, ", "))
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
