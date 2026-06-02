package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

type staticClass int

const (
	staticRevalidate staticClass = iota
	staticImmutable
	staticLong
)

const (
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheLong       = "public, max-age=2592000"
	cacheRevalidate = "public, max-age=0, must-revalidate"
	cacheDev        = "no-cache"
)

func StaticCache(next http.Handler, devMode bool, etags map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if devMode {
			w.Header().Set("Cache-Control", cacheDev)
			next.ServeHTTP(w, r)

			return
		}

		switch classifyStatic(r.URL.Path) {
		case staticImmutable:
			w.Header().Set("Cache-Control", cacheImmutable)
		case staticLong:
			w.Header().Set("Cache-Control", cacheLong)
		default:
			w.Header().Set("Cache-Control", cacheRevalidate)
			if etag, ok := etags[r.URL.Path]; ok {
				w.Header().Set("Etag", etag)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func classifyStatic(path string) staticClass {
	if strings.Contains(path, "/vendor/") || strings.Contains(path, "/fonts/") {
		return staticImmutable
	}

	if strings.HasPrefix(path, "/static/item/") ||
		strings.HasPrefix(path, "/static/mob/") ||
		strings.HasPrefix(path, "/static/collection/") {
		return staticLong
	}

	return staticRevalidate
}

func StaticETags(files fs.FS, urlPrefix string) (map[string]string, error) {
	etags := make(map[string]string)

	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(files, path)
		if err != nil {
			return fmt.Errorf("httpx.StaticETags: read %s: %w", path, err)
		}

		sum := sha256.Sum256(data)
		etags[urlPrefix+path] = `"` + hex.EncodeToString(sum[:16]) + `"`

		return nil
	}

	if err := fs.WalkDir(files, ".", walk); err != nil {
		return nil, fmt.Errorf("httpx.StaticETags: walk: %w", err)
	}

	return etags, nil
}
