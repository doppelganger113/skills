// Package web holds HTTP plumbing shared by every domain: JSON encoding,
// a single error envelope, middleware and the server lifecycle. It
// contains no business rules and imports no domain package.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// maxBodyBytes caps request bodies so a client cannot exhaust memory.
const maxBodyBytes = 1 << 20

// JSON writes v as a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status is already sent; an encoding error here can only be a
	// broken connection, which there is no one left to report to.
	_ = json.NewEncoder(w).Encode(v)
}

// problem is the one error body every domain uses (RFC 9457 shape plus a
// stable machine-readable code), so clients see a consistent envelope no
// matter how many domains the API grows to.
type problem struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Problem writes an error response. code is stable and namespaced by
// domain ("order.not_found"); detail is safe, human-readable text.
func Problem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Title:  http.StatusText(status),
		Status: status,
		Code:   code,
		Detail: detail,
	})
}

// InternalError logs err with request context and writes a 500 that
// reveals nothing about it. Every domain's error mapper ends here.
func InternalError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	logger.ErrorContext(r.Context(), "request failed", "err", err)
	Problem(w, http.StatusInternalServerError, "internal", "")
}

var errTrailingData = errors.New("unexpected data after JSON object")

// Decode reads a JSON request body into v, rejecting unknown fields,
// trailing data and bodies over maxBodyBytes. On failure it writes the
// error response itself and returns false.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil && dec.More() {
		err = errTrailingData
	}
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		Problem(w, http.StatusRequestEntityTooLarge, "request.too_large", "request body too large")
	} else {
		Problem(w, http.StatusBadRequest, "request.malformed", decodeDetail(err))
	}
	return false
}

// decodeDetail turns an encoding/json error into a client-safe message
// that names the field but not Go types.
func decodeDetail(err error) string {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.Is(err, errTrailingData):
		return "request body has data after the JSON object"
	case errors.Is(err, io.EOF):
		return "request body is empty"
	case errors.Is(err, io.ErrUnexpectedEOF), errors.As(err, &syntaxErr):
		return "request body is not valid JSON"
	case errors.As(err, &typeErr) && typeErr.Field != "":
		return fmt.Sprintf("field %q has the wrong type", typeErr.Field)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json has no typed error for this; the message names
		// only the field, so it is safe to pass on.
		return "unknown field " + strings.TrimPrefix(err.Error(), "json: unknown field ")
	default:
		return "request body is invalid"
	}
}

// Serve runs an HTTP server on ln until ctx is cancelled, then shuts it
// down gracefully, giving in-flight requests up to 10 seconds to finish.
// Taking a listener rather than an address lets callers (and tests) bind
// first and know the real address.
func Serve(ctx context.Context, ln net.Listener, h http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	logger.InfoContext(ctx, "http server listening", "addr", ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down http server: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}
	logger.InfoContext(ctx, "http server stopped")
	return nil
}
