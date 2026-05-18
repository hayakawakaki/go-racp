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
)

const (
	cacheDirPerm  = 0o755
	cacheFilePerm = 0o644
)

type Cache[T any] struct {
	Logger   *slog.Logger
	Dir      string
	Filename string
	Version  int
}

type sourceHash struct {
	Path string
	Hash uint64
}

//nolint:govet // generic instantiation pays the alignment cost; readability wins here
type cacheBlob[T any] struct {
	Sources []sourceHash
	Value   T
	Version int
}

func (c Cache[T]) path() string {
	return filepath.Join(c.Dir, c.Filename)
}

func (c Cache[T]) Load(paths []string) (T, bool) {
	var zero T
	logger := c.loggerOrDefault()

	//nolint:gosec // G304: c.Dir is set by the caller from a trusted project path.
	file, err := os.Open(c.path())
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("refdata: cache open failed", "err", err)
		}

		return zero, false
	}
	defer func() { _ = file.Close() }()

	var blob cacheBlob[T]
	if err = gob.NewDecoder(file).Decode(&blob); err != nil {
		logger.Warn("refdata: cache decode failed", "err", err)

		return zero, false
	}
	if blob.Version != c.Version {
		return zero, false
	}

	current, err := hashSources(paths)
	if err != nil {
		logger.Warn("refdata: source hash failed", "err", err)

		return zero, false
	}
	if !sameHashes(blob.Sources, current) {
		return zero, false
	}

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

	blob := cacheBlob[T]{Version: c.Version, Sources: hashes, Value: value}
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
		sum, err := hashFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return nil, err
		}
		out = append(out, sourceHash{Path: path, Hash: sum})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	return out, nil
}

func hashFile(path string) (uint64, error) {
	//nolint:gosec // G304: path comes from operator-controlled config paths.
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

func sameHashes(a, b []sourceHash) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}

	return true
}
