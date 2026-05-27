package cli

import (
	"errors"
	"fmt"
)

const (
	// ExitSuccess indicates successful command completion.
	ExitSuccess = 0
	// ExitInternal indicates an internal, configuration, input, or output error.
	ExitInternal = 1
	// ExitSQLSyntax indicates rejected SQL or lint issues.
	ExitSQLSyntax = 2
	// ExitConnection indicates database connection setup failure.
	ExitConnection = 3
	// ExitTimeout indicates command execution exceeded its timeout.
	ExitTimeout = 4
	// ExitCancelled indicates command execution was cancelled by the user.
	ExitCancelled = 5
)

// CommandError carries a machine-readable error type and process exit code.
type CommandError struct {
	Type     string
	Message  string
	Position int
	Code     int
}

// Error returns the user-facing error message.
func (e *CommandError) Error() string {
	if e.Message == "" {
		return e.Type
	}
	return e.Message
}

// ExitCode maps an error to sqio's process exit code convention.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		return commandErr.Code
	}
	return ExitInternal
}

// StructuredError formats err as a compact JSON object for stderr output.
func StructuredError(err error) string {
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		commandErr = &CommandError{Type: "internal", Message: err.Error(), Code: ExitInternal}
	}
	if commandErr.Position > 0 {
		return fmt.Sprintf(`{"error":{"type":%q,"message":%q,"position":%d}}`, commandErr.Type, commandErr.Message, commandErr.Position)
	}
	return fmt.Sprintf(`{"error":{"type":%q,"message":%q}}`, commandErr.Type, commandErr.Message)
}
