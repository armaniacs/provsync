// Package cli はサブコマンドの解析と実行を担う。
package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/armaniacs/provsync/internal/adapter"
	"github.com/armaniacs/provsync/internal/backup"
	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/lock"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/version"
)

type options struct {
	out         io.Writer
	errOut      io.Writer
	lang        string
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

// T は実行時の言語でカタログ文言を返す。
func (o *options) T(id string, a ...any) string { return i18n.T(o.lang, id, a...) }

// msgf はカタログ文言を 1 行として writer へ出す。
func (o *options) msgf(w io.Writer, id string, a ...any) {
	fmt.Fprintln(w, o.T(id, a...))
}

// warnf は警告を "警告: <本文>" の形で errOut へ出す。
func (o *options) warnf(id string, a ...any) {
	fmt.Fprintf(o.errOut, "%s: %s\n", o.T("label.warning"), o.T(id, a...))
}

// warnm は内部パッケージの Message を "警告: <訳>" の形で errOut へ出す。
func (o *options) warnm(m *i18n.Message) {
	fmt.Fprintf(o.errOut, "%s: %s\n", o.T("label.warning"), i18n.Localize(o.lang, m))
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
	opts := &options{out: out, errOut: errOut, keep: backup.MaxOperations, lang: i18n.ResolveFromEnv()}
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
	if opts.help && len(pos) > 0 {
		if c, ok := lookupCommand(pos[0]); ok {
			fmt.Fprint(out, i18n.T(opts.lang, c.helpKey))
			return nil
		}
	}
	if opts.help || len(pos) == 0 {
		printUsage(opts)
		return nil
	}

	cmd, rest := pos[0], pos[1:]
	if c, ok := lookupCommand(cmd); ok {
		return c.handler(opts, rest)
	}
	return opts.usageErr("err.unknownCommand", cmd)
}

func registerFlags(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.rootFlag, "root", o.rootFlag, o.T("flag.root"))
	fs.BoolVar(&o.write, "write", o.write, o.T("flag.write"))
	fs.BoolVar(&o.noBackup, "no-backup", o.noBackup, o.T("flag.noBackup"))
	fs.Var(&o.providers, "provider", o.T("flag.provider"))
	fs.StringVar(&o.from, "from", o.from, o.T("flag.from"))
	fs.StringVar(&o.to, "to", o.to, o.T("flag.to"))
	fs.BoolVar(&o.list, "list", o.list, o.T("flag.list"))
	fs.BoolVar(&o.prune, "prune", o.prune, o.T("flag.prune"))
	fs.IntVar(&o.keep, "keep", o.keep, o.T("flag.keep"))
	fs.BoolVar(&o.jsonOut, "json", o.jsonOut, o.T("flag.json"))
	fs.BoolVar(&o.exitCode, "exit-code", o.exitCode, o.T("flag.exitCode"))
	fs.BoolVar(&o.strict, "strict", o.strict, o.T("flag.strict"))
	fs.BoolVar(&o.version, "version", o.version, o.T("flag.version"))
	fs.BoolVar(&o.showSecrets, "show-secrets", o.showSecrets, o.T("flag.showSecrets"))
	fs.BoolVar(&o.help, "help", o.help, o.T("flag.help"))
	fs.BoolVar(&o.help, "h", o.help, o.T("flag.help"))
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

// ---- 共通 ----

// keepFromEnv はバックアップ保持数を環境変数 PROVSYNC_KEEP から読む。
// 未設定は backup.Store の既定値を使う。1 未満・非数はエラー。
// エラーは言語中立の Message で返し、描画(main の Localize)が言語を決める。
func keepFromEnv() (int, error) {
	v := os.Getenv("PROVSYNC_KEEP")
	if v == "" {
		return backup.MaxOperations, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, i18n.New("err.keepEnv.invalid", v)
	}
	return n, nil
}

// newBackupStore は保持数の環境変数を反映したバックアップ Store を返す。
// applyOrPreview と cmdUndo の 2 経路で共有する。
func newBackupStore(root adapter.Root) (*backup.Store, error) {
	st := backup.New(root.StateDir())
	max, err := keepFromEnv()
	if err != nil {
		return nil, err
	}
	st.SetMax(max)
	return st, nil
}

func (o *options) applyOrPreview(label string, p plan.Plan) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	renderPreview(o, p)
	if !p.Changed() {
		o.msgf(o.out, "msg.noChanges")
		return nil
	}
	if !o.write {
		o.msgf(o.out, "msg.previewOnly")
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
		st, err := newBackupStore(root)
		if err != nil {
			return err
		}
		op, err := st.Record(label, paths)
		if err != nil {
			return i18n.Wrap(err, "err.backup.failed")
		}
		o.msgf(o.out, "msg.backupID", op.ID)
	} else if _, err := backup.New(root.StateDir()).RecordMarker(label); err != nil {
		return i18n.Wrap(err, "err.recordHistory.failed")
	}
	written := make([]string, 0, len(changed))
	for _, c := range changed {
		if err := fsutil.WriteFileAtomic(c.Path, c.After); err != nil {
			if len(written) > 0 {
				fmt.Fprintf(o.errOut, "%s: %s\n", o.T("label.warning"), o.T("msg.writtenFiles", strings.Join(written, ", ")))
				if !o.noBackup {
					o.msgf(o.out, "msg.undoHint")
				}
			}
			return i18n.Wrap(err, "err.write.failed", c.Path)
		}
		written = append(written, c.Path)
		o.msgf(o.out, "msg.written", c.Path)
	}
	return nil
}
