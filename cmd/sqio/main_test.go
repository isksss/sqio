package main

import (
	"os"
	"testing"
)

func TestMainSuccess(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"sqio", "--quiet", "exec", "--sql", "select 1"}
	if code := run(); code != 0 {
		t.Fatalf("expected success exit, got %d", code)
	}
}

func TestMainCallsExit(t *testing.T) {
	oldArgs := os.Args
	oldExit := exit
	t.Cleanup(func() {
		os.Args = oldArgs
		exit = oldExit
	})
	os.Args = []string{"sqio", "--quiet", "exec", "--sql", "select 1"}
	var got int
	exit = func(code int) {
		got = code
	}
	main()
	if got != 0 {
		t.Fatalf("expected success exit, got %d", got)
	}
}

func TestRunError(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"sqio", "missing-command"}
	if code := run(); code == 0 {
		t.Fatal("expected non-zero exit")
	}
}
