package main

import (
	"os"
	"testing"
)

func TestMainSuccess(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"sqio", "--quiet", "exec", "--sql", "select 1"}
	main()
}
