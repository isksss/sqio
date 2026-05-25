package query

import (
	"errors"
	"io"
	"os"
	"strings"
)

var ErrMultipleInputs = errors.New("specify only one of --sql, --file, or stdin")

type Source struct {
	SQL  string
	File string
	In   io.Reader
}

func Read(src Source) (string, error) {
	count := 0
	if src.SQL != "" {
		count++
	}
	if src.File != "" {
		count++
	}
	hasStdin := hasInput(src.In)
	if hasStdin {
		count++
	}
	if count > 1 {
		return "", ErrMultipleInputs
	}
	switch {
	case src.SQL != "":
		return src.SQL, nil
	case src.File != "":
		b, err := os.ReadFile(src.File)
		return string(b), err
	case hasStdin:
		b, err := io.ReadAll(src.In)
		return string(b), err
	default:
		return "", nil
	}
}

func Statements(sql string) []string {
	parts := strings.Split(sql, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			statements = append(statements, trimmed)
		}
	}
	return statements
}

func hasInput(r io.Reader) bool {
	if r == nil {
		return false
	}
	if file, ok := r.(*os.File); !ok || file != os.Stdin {
		return true
	}
	return stdinHasData()
}

func stdinHasData() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}
