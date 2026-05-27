// Package main contains the sqio command entrypoint.
package main

import (
	"os"

	"github.com/isksss/sqio/internal/cli"
)

// main delegates command execution to the CLI package and exits with the
// structured exit code derived from any returned error.
func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
