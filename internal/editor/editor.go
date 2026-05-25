package editor

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Select() string {
	for _, key := range []string{"DBTUI_EDITOR", "VISUAL", "EDITOR"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return "vi"
}

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
