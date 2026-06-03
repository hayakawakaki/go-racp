package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

		next.ServeHTTP(&cacheableWriter{ResponseWriter: w}, r)
	})
}

type cacheableWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (c *cacheableWriter) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}

	c.wroteHeader = true

	if !cacheableStatus(status) {
		c.Header().Del("Cache-Control")
		c.Header().Del("Etag")
	}

	c.ResponseWriter.WriteHeader(status)
}

func (c *cacheableWriter) ReadFrom(src io.Reader) (int64, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}

	written, err := io.Copy(c.ResponseWriter, src)
	if err != nil {
		return written, fmt.Errorf("httpx.cacheableWriter.ReadFrom: %w", err)
	}

	return written, nil
}

func (c *cacheableWriter) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}

func cacheableStatus(status int) bool {
	return status == http.StatusOK ||
		status == http.StatusPartialContent ||
		status == http.StatusNotModified
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
