package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestCommandErrorFormatting(t *testing.T) {
	if got := (&CommandError{Type: "config"}).Error(); got != "config" {
		t.Fatalf("unexpected error: %s", got)
	}
	err := &CommandError{Type: "lint", Message: "bad sql", Position: 3, Code: ExitSQLSyntax}
	if ExitCode(err) != ExitSQLSyntax {
		t.Fatalf("unexpected exit code")
	}
	if got := StructuredError(err); !strings.Contains(got, `"position":3`) {
		t.Fatalf("unexpected structured error: %s", got)
	}
	if got := StructuredError(errors.New("plain")); !strings.Contains(got, `"type":"internal"`) {
		t.Fatalf("unexpected generic error: %s", got)
	}
}
