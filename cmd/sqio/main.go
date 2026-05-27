// Package main contains the sqio command entrypoint.
package main

import (
	"os"

	"github.com/isksss/sqio/internal/cli"
)

var exit = os.Exit

// main delegates command execution to the CLI package and exits with the
// structured exit code derived from any returned error.
func main() {
	exit(run())
}

func run() int {
	if err := cli.Execute(); err != nil {
		return cli.ExitCode(err)
	}
	return cli.ExitSuccess
}
