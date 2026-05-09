package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func GetTargetFilePath(target string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		// Check for target in the current directory
		path := filepath.Join(dir, target)
		info, statErr := os.Stat(path)
		switch {
		case statErr == nil && !info.IsDir():
			return path, nil
		case statErr != nil && !errors.Is(statErr, fs.ErrNotExist):
			return "", fmt.Errorf("stat %s: %w", path, statErr)
		}

		// Make sure we don't crawl outside the project
		_, statErr = os.Stat(filepath.Join(dir, "go.mod"))
		switch {
		case statErr == nil:
			return "", fmt.Errorf("target file %s was not found", target)
		case !errors.Is(statErr, fs.ErrNotExist):
			return "", fmt.Errorf("stat go.mod: %w", statErr)
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
