package web

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type requestIDKey struct{}

// RequestID returns the request ID stored in ctx, or "" if there is none.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// WithRequestID tags each request with an ID, taken from the
// X-Request-ID header when it is well formed, and echoes it in the
// response.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = rand.Text()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// validRequestID accepts client-supplied IDs only if they are short and
// made of characters that are safe in logs and headers.
func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.'
		if !ok {
			return false
		}
	}
	return true
}

// Recover turns a panic in a handler into a logged 500 instead of a
// dropped connection. If the handler had already started the response,
// it can only log and abort.
func Recover(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v) // deliberate abort; let net/http handle it
			}
			logger.ErrorContext(r.Context(), "panic in handler", "panic", v, "stack", string(debug.Stack()))
			if rec.wroteHeader {
				panic(http.ErrAbortHandler) // too late for a 500; drop the connection
			}
			Problem(w, http.StatusInternalServerError, "internal", "")
		}()
		next.ServeHTTP(rec, r)
	})
}

// LogRequests logs method, path, status and duration for every request.
// It logs in a defer so even an aborted request (a panic that Recover
// re-raises) gets its line.
func LogRequests(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			logger.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", time.Since(start),
			)
		}()
		next.ServeHTTP(rec, r)
	})
}

// statusRecorder remembers the status and whether the response started.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer for
// flushing, deadlines and hijacking.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// ContextHandler is a slog.Handler that adds request-scoped attributes
// from the context to every record. The standard handlers ignore ctx,
// so without this, InfoContext(ctx, ...) would not show the request ID.
type ContextHandler struct {
	slog.Handler
}

// Handle adds request_id when ctx carries one.
func (h ContextHandler) Handle(ctx context.Context, rec slog.Record) error {
	if id := RequestID(ctx); id != "" {
		rec.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, rec)
}

// WithAttrs keeps the wrapper when attributes are added.
func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextHandler{h.Handler.WithAttrs(attrs)}
}

// WithGroup keeps the wrapper when a group is opened.
func (h ContextHandler) WithGroup(name string) slog.Handler {
	return ContextHandler{h.Handler.WithGroup(name)}
}
