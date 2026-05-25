package picker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func SQLFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".sql") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func Pick(options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no candidates")
	}
	if fzf, err := exec.LookPath("fzf"); err == nil {
		cmd := exec.Command(fzf)
		cmd.Stdin = strings.NewReader(strings.Join(options, "\n"))
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			selected := strings.TrimSpace(out.String())
			if selected != "" {
				return selected, nil
			}
		}
	}
	return options[0], nil
}
