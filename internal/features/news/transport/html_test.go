package transport

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
)

func TestParseID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pathVal string
		wantID  int64
		wantOK  bool
	}{
		{"positive", "42", 42, true},
		{"large", "9223372036854775807", 9223372036854775807, true},
		{"zero", "0", 0, false},
		{"negative", "-1", 0, false},
		{"non-numeric", "abc", 0, false},
		{"empty", "", 0, false},
		{"float", "3.14", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.SetPathValue("id", tt.pathVal)

			gotID, gotOK := parseID(req)
			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Errorf("parseID(id=%q) = (%d, %v), want (%d, %v)",
					tt.pathVal, gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestFieldFromErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err       error
		name      string
		wantField string
		wantMsg   string
	}{
		{domain.ErrTitleEmpty, "title empty", fieldTitle, "Title is required"},
		{domain.ErrTitleTooLong, "title too long", fieldTitle, "Title is too long"},
		{domain.ErrBodyEmpty, "body empty", fieldBody, "Body is required"},
		{domain.ErrBodyTooLong, "body too long", fieldBody, "Body is too long"},
		{domain.ErrInvalidCategory, "invalid category", fieldCategory, "Invalid category"},
		{errors.New("other"), "unknown error", "", ""},
		{nil, "nil error", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotField, gotMsg := fieldFromErr(tt.err)
			if gotField != tt.wantField || gotMsg != tt.wantMsg {
				t.Errorf("fieldFromErr(%v) = (%q, %q), want (%q, %q)",
					tt.err, gotField, gotMsg, tt.wantField, tt.wantMsg)
			}
		})
	}
}
