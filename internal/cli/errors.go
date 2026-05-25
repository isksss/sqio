package cli

import (
	"errors"
	"fmt"
)

const (
	ExitSuccess    = 0
	ExitInternal   = 1
	ExitSQLSyntax  = 2
	ExitConnection = 3
	ExitTimeout    = 4
	ExitCancelled  = 5
)

type CommandError struct {
	Type     string
	Message  string
	Position int
	Code     int
}

func (e *CommandError) Error() string {
	if e.Message == "" {
		return e.Type
	}
	return e.Message
}

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
