package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/armaniacs/provsync/internal/cli"
)

func main() {
	if err := cli.RunWith(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "provsync:", err)
		var ue *cli.UsageError
		if errors.As(err, &ue) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
