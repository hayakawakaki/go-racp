package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const adminRoleName = "Admin"

// ProjectRoot walks up from the current working directory and returns the first ancestor containing go.mod.
func ProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil {
			return dir, nil
		}
		if !errors.Is(statErr, fs.ErrNotExist) {
			return "", fmt.Errorf("stat go.mod: %w", statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root (go.mod): %w", fs.ErrNotExist)
		}
		dir = parent
	}
}

// GetTargetFilePath walks up from the current working directory to find target by name, stopping at the directory containing go.mod.
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
			return "", fmt.Errorf("target file %s: %w", target, fs.ErrNotExist)
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

	return "", fmt.Errorf("target file %s: %w", target, fs.ErrNotExist)
}
