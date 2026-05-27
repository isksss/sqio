// Package editor opens SQL text in the user's preferred external editor.
package editor

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Select chooses the editor executable from environment variables, falling back
// to vi when no preference is configured.
func Select() string {
	for _, key := range []string{"DBTUI_EDITOR", "VISUAL", "EDITOR"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return "vi"
}

// Edit writes initial SQL into a temporary file, opens it in the selected
// editor, and returns the edited contents.
func Edit(initial string) (string, error) {
	dir := filepath.Join(os.TempDir(), "sqio")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "query-*.sql")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(initial); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	cmd := exec.Command(Select(), path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	return string(b), err
}
