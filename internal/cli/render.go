// Package cli のうち、出力の描画を担う。
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/armaniacs/provsync/internal/plan"
)

// encodeJSON は v を 2 スペースインデントの JSON で出力する。
func encodeJSON(out io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
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
