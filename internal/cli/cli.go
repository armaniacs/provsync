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
	"github.com/armaniacs/provsync/internal/lock"
	"github.com/armaniacs/provsync/internal/plan"
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
	if opts.help && len(pos) > 0 {
		if c, ok := lookupCommand(pos[0]); ok {
			fmt.Fprint(out, c.help)
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
	return usageErr("未知のコマンド %q です(--help を参照)", cmd)
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
		st, err := newBackupStore(root)
		if err != nil {
			return err
		}
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
