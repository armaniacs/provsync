// Package cli のうち、version 表示を担う。
package cli

import (
	"fmt"
	"io"

	"github.com/armaniacs/provsync/internal/version"
)

// printVersion writes the one-line version banner in a single format.
func printVersion(out io.Writer) {
	fmt.Fprintf(out, "provsync %s\n", version.String())
}

// cmdVersion はバージョンを 1 行で表示する。
func cmdVersion(o *options, args []string) error {
	printVersion(o.out)
	return nil
}
