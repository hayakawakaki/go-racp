package refdata

import (
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	cacheDirPerm  = 0o755
	cacheFilePerm = 0o644
)

type Cache[T any] struct {
	Logger   *slog.Logger
	Dir      string
	Filename string
}

type sourceHash struct {
	Path  string
	Hash  uint64
	Mtime int64
	Size  int64
}

//nolint:govet // generic instantiation
type cacheBlob[T any] struct {
	Sources []sourceHash
	Value   T
}

func (c Cache[T]) path() string {
	return filepath.Join(c.Dir, c.Filename)
}

func (c Cache[T]) Load(paths []string) (T, bool) {
	var zero T
	logger := c.loggerOrDefault()

	//nolint:gosec // G304: c.Dir is set by the config
	file, err := os.Open(c.path())
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("refdata: cache open failed", "err", err)
		}
		return zero, false
	}
	defer func() { _ = file.Close() }()

	var blob cacheBlob[T]
	decodeStart := time.Now()
	if err = gob.NewDecoder(file).Decode(&blob); err != nil {
		logger.Warn("refdata: cache decode failed", "err", err)
		return zero, false
	}
	decodeTook := time.Since(decodeStart)

	verifyStart := time.Now()
	if !verifySources(blob.Sources, paths) {
		return zero, false
	}
	verifyTook := time.Since(verifyStart)

	logger.Info("refdata: cache hit", "file", c.path(), "decode", decodeTook.String(), "verify", verifyTook.String())

	return blob.Value, true
}

func (c Cache[T]) Save(value T, paths []string) error {
	if err := os.MkdirAll(c.Dir, cacheDirPerm); err != nil {
		return fmt.Errorf("refdata.Cache.Save mkdir: %w", err)
	}

	hashes, err := hashSources(paths)
	if err != nil {
		return fmt.Errorf("refdata.Cache.Save hash: %w", err)
	}

	target := c.path()
	file, err := os.CreateTemp(c.Dir, c.Filename+".*.tmp")
	if err != nil {
		return fmt.Errorf("refdata.Cache.Save create: %w", err)
	}
	tmp := file.Name()
	if err := file.Chmod(cacheFilePerm); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("refdata.Cache.Save chmod: %w", err)
	}

	blob := cacheBlob[T]{Sources: hashes, Value: value}
	if err := gob.NewEncoder(file).Encode(blob); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("refdata.Cache.Save encode: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("refdata.Cache.Save close: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("refdata.Cache.Save rename: %w", err)
	}

	return nil
}

func (c Cache[T]) loggerOrDefault() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}

	return slog.Default()
}

func hashSources(paths []string) ([]sourceHash, error) {
	out := make([]sourceHash, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		sum, err := hashFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, sourceHash{
			Path:  path,
			Hash:  sum,
			Mtime: info.ModTime().Unix(),
			Size:  info.Size(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	return out, nil
}

func verifySources(cached []sourceHash, paths []string) bool {
	byPath := make(map[string]sourceHash, len(cached))
	for _, entry := range cached {
		byPath[entry.Path] = entry
	}

	matched := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return false
		}
		prev, ok := byPath[path]
		if !ok {
			return false
		}
		if prev.Mtime == info.ModTime().Unix() && prev.Size == info.Size() {
			matched++

			continue
		}
		sum, err := hashFile(path)
		if err != nil {
			return false
		}
		if sum != prev.Hash {
			return false
		}
		matched++
	}

	return matched == len(cached)
}

func hashFile(path string) (uint64, error) {
	//nolint:gosec // G304: path comes from config
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	hasher := fnv.New64a()
	if _, err := io.Copy(hasher, file); err != nil {
		return 0, fmt.Errorf("hash %s: %w", path, err)
	}

	return hasher.Sum64(), nil
}
