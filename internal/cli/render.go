// Package cli のうち、出力の描画を担う。
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/armaniacs/provsync/internal/i18n"
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
func warnLoosePerm(o *options, path string, dir bool) {
	info, err := os.Stat(path)
	if err != nil || !permLoose(info.Mode().Perm()) {
		return
	}
	perm := info.Mode().Perm()
	suggest := "600"
	if dir {
		suggest = "700"
	}
	fmt.Fprintf(o.errOut, "%s: %s\n", o.T("label.warning"), o.T("warn.loosePerm", perm, path, suggest, path))
}

func renderPreview(o *options, p plan.Plan) {
	for _, c := range p.Changes {
		fmt.Fprintf(o.out, "%s: %s\n", c.Tool, c.Path)
		renderSemantic(o, c.Semantic)
	}
}

func renderSemantic(o *options, changes []plan.ProviderChange) {
	shown := 0
	for _, ch := range changes {
		if ch.Op == "unchanged" {
			continue
		}
		shown++
		if len(ch.Fields) > 0 {
			fmt.Fprintf(o.out, "  %s: %s (%s)\n", ch.Key, opLabel(o.lang, ch.Op), strings.Join(ch.Fields, ", "))
		} else {
			fmt.Fprintf(o.out, "  %s: %s\n", ch.Key, opLabel(o.lang, ch.Op))
		}
	}
	if shown == 0 {
		o.msgf(o.out, "msg.noChange")
	}
}

func opLabel(lang, op string) string {
	switch op {
	case "added":
		return i18n.T(lang, "label.added")
	case "updated":
		return i18n.T(lang, "label.updated")
	default:
		return i18n.T(lang, "label.unchanged")
	}
}
