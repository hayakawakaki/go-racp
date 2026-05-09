package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetTargetFilePath(target string) (string, error) {
	dir, _ := os.Getwd()

	for {
		// Check for target in the current directory
		path := filepath.Join(dir, target)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}

		// Make sure we don't crawl outside the project
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}

		// Move to the parent directory
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			break
		}
		dir = parentDir
	}

	return "", fmt.Errorf("target file %s was not found", target)
}
