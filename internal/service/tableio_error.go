package service

import "fmt"

func errUnsupportedImportFormat(format string) error {
	return fmt.Errorf("unsupported import format: %s", format)
}
