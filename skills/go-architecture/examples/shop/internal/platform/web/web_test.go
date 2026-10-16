package web_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/shop/internal/platform/web"
)

func TestDecodeHidesGoDetails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, wantDetail string
	}{
		{"empty", "", "request body is empty"},
		{"syntax", `{"n":`, "request body is not valid JSON"},
		{"wrong type", `{"n":"x"}`, `field "n" has the wrong type`},
		{"unknown field", `{"m":1}`, `unknown field "m"`},
		{"trailing data", `{"n":1}{}`, "request body has data after the JSON object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), "POST", "/", strings.NewReader(tt.body))
			var v struct {
				N int `json:"n"`
			}
			if web.Decode(rec, req, &v) {
				t.Fatal("Decode() = true, want false")
			}
			var p struct{ Detail string }
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", p.Detail, tt.wantDetail)
			}
		})
	}
}

func TestRecoverReturns500(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := web.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}), logger)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRecoverAbortsWhenResponseStarted(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := web.Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("boom")
	}), logger)

	defer func() {
		v := recover()
		if err, ok := v.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Errorf("recovered %v, want http.ErrAbortHandler", v)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), "GET", "/", nil))
}

func TestWithRequestIDRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()
	var seen string
	h := web.WithRequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = web.RequestID(r.Context())
	}))

	for _, tt := range []struct {
		header   string
		wantKept bool
	}{
		{"abc-123_x.y", true},
		{"bad id\nwith newline", false},
		{strings.Repeat("a", 200), false},
	} {
		req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
		req.Header.Set("X-Request-ID", tt.header)
		h.ServeHTTP(httptest.NewRecorder(), req)
		if kept := seen == tt.header; kept != tt.wantKept || seen == "" {
			t.Errorf("header %q: request ID = %q, want kept=%v and non-empty", tt.header, seen, tt.wantKept)
		}
	}
}
