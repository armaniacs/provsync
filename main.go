package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/armaniacs/provsync/internal/cli"
	"github.com/armaniacs/provsync/internal/i18n"
)

func main() {
	// 言語判定はプロセス入口で 1 回。RunWith も同じ env から同じ結果を解決する。
	lang := i18n.ResolveFromEnv()
	if err := cli.Supported(runtime.GOOS); err != nil {
		fmt.Fprintln(os.Stderr, "provsync:", i18n.Localize(lang, err))
		os.Exit(1)
	}
	if err := cli.RunWith(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var ee *cli.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, "provsync:", i18n.Localize(lang, err))
		var ue *cli.UsageError
		if errors.As(err, &ue) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
