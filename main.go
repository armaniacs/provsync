package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/armaniacs/provsync/internal/cli"
)

func main() {
	if err := cli.Supported(runtime.GOOS); err != nil {
		fmt.Fprintln(os.Stderr, "provsync:", err)
		os.Exit(1)
	}
	if err := cli.RunWith(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var ee *cli.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, "provsync:", err)
		var ue *cli.UsageError
		if errors.As(err, &ue) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
