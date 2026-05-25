package main

import (
	"os"

	"github.com/isksss/sqio/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
