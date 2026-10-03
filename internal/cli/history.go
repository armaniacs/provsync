// Package cli のうち、diff と undo を担う。
package cli

import (
	"fmt"
	"time"

	"github.com/armaniacs/provsync/internal/diff"
	"github.com/armaniacs/provsync/internal/lock"
	"github.com/armaniacs/provsync/internal/plan"
	"github.com/armaniacs/provsync/internal/secret"
)

// ---- diff ----

func cmdDiff(o *options, args []string) error {
	if len(args) != 2 {
		return o.usageErr("err.usage.diff")
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
		renderSemantic(o, c.Semantic)
		d := diff.Unified(c.Path, c.Before, c.After, 3)
		if d == "" {
			o.msgf(o.out, "msg.noChange")
			continue
		}
		if !o.showSecrets {
			d = secret.MaskLines(d)
		} else {
			fmt.Fprintln(o.errOut, o.T("label.warning")+": "+o.T("warn.showSecrets"))
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
	st, err := newBackupStore(root)
	if err != nil {
		return err
	}

	if o.prune {
		removed, err := st.Prune(o.keep)
		if err != nil {
			return err
		}
		o.msgf(o.out, "msg.pruned", removed)
		return nil
	}

	if o.list {
		ops, err := st.List()
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			o.msgf(o.out, "msg.noHistory")
			return nil
		}
		for _, op := range ops {
			status := ""
			if op.NoBackup {
				status = o.T("msg.list.noBackup")
			} else if op.Undone {
				status = o.T("msg.list.undone")
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
			fmt.Fprintln(o.errOut, o.T("label.warning")+": "+o.T("warn.undo.noBackup"))
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
	o.msgf(o.out, "msg.restored", op.ID, op.Command)
	o.msgf(o.out, "msg.redo", undoOp.ID)
	for _, f := range op.Files {
		if f.Existed {
			o.msgf(o.out, "msg.undo.restored", f.Path)
		} else {
			o.msgf(o.out, "msg.undo.removed", f.Path)
		}
	}
	return nil
}
