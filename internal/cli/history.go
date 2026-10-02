// Package cli のうち、diff と undo を担う。
package cli

import (
	"fmt"
	"time"

	"github.com/armaniacs/provsync/internal/backup"
	"github.com/armaniacs/provsync/internal/diff"
	"github.com/armaniacs/provsync/internal/lock"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/secret"
)

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
