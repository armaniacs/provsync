package main

import (
	"fmt"
	"os"

	"github.com/armaniacs/provsync/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "provsync:", err)
		os.Exit(1)
	}
}
