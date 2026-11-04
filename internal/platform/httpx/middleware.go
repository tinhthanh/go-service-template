// Package httpx is the middleware every request goes through, on plain
// net/http: a request id, a log line, recovery from a panic, security
// headers, CORS for the client's origins, and the fields a PATCH sends as
// null (Nulls). Nothing here knows a feature.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// Middleware wraps a handler.
type Middleware func(http.Handler) http.Handler

// Chain applies the middleware in the order given: the first is outermost.
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for _, m := range slices.Backward(mw) {
		h = m(h)
	}

	return h
}

type requestIDKey struct{}

const _requestIDBytes = 8

// RequestID gives every request an id: the caller's X-Request-ID, or a new one.
// It is on the answer and on every log line of the request.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" || len(id) > 64 {
				b := make([]byte, _requestIDBytes)
				_, _ = rand.Read(b)
				id = hex.EncodeToString(b)
			}

			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
		})
	}
}

// RequestIDOf is the request's id, or "".
func RequestIDOf(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)

	return id
}

// statusWriter remembers the status a handler wrote, for the log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Log writes one line per request: method, path, status, how long, its id.
// Never the query or the body: they can carry what a person typed.
func Log(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.InfoContext(r.Context(), "request",
				"method", r.Method, "path", r.URL.Path, "status", sw.status,
				"ms", time.Since(start).Milliseconds(), "request_id", RequestIDOf(r.Context()))
		})
	}
}

// Recover turns a panic into a 500 in the contract's shape, logged with its value.
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer recovered(w, r, log)
			next.ServeHTTP(w, r)
		})
	}
}

// recovered answers a panic that reached it, if one did.
func recovered(w http.ResponseWriter, r *http.Request, log *slog.Logger) {
	if v := recover(); v != nil {
		log.ErrorContext(r.Context(), "panic", "value", v, "request_id", RequestIDOf(r.Context()))
		problem.Write(w, r, log, errPanic)
	}
}

// SecurityHeaders are the headers an API answer always carries.
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		})
	}
}

// CORS lets the named origins call the API with their cookies. An origin
// not named gets no CORS headers, so the browser refuses it; "*" is never
// allowed, since a credentialed request can't use it.
func CORS(origins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" || !slices.Contains(origins, origin) {
				next.ServeHTTP(w, r)

				return
			}

			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE")
				h.Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With, X-Request-ID")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
