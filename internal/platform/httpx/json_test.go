package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON_HappyPath(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	payload := map[string]string{"hello": "world"}

	if err := WriteJSON(rr, http.StatusOK, payload); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}

	var got map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["hello"] != "world" {
		t.Errorf("body = %v", got)
	}
}

func TestWriteJSON_SetsStatus(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	_ = WriteJSON(rr, http.StatusNotFound, map[string]string{"error": "missing"})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "missing") {
		t.Errorf("body = %q, want substring missing", rr.Body.String())
	}
}

func TestWriteJSON_PropagatesEncodeError(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	if err := WriteJSON(rr, http.StatusOK, make(chan int)); err == nil {
		t.Errorf("expected error encoding chan payload, got nil")
	}
}
