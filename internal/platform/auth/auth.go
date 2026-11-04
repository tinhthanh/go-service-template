// Package auth is who is calling and what they may do: the Principal a
// session cookie names (found by an Authenticator, which the session
// feature is), on the request's context for every handler; the CSRF rule
// for writes; and the contract's x-permission and x-roles (Rules). It knows
// no feature: internal/server passes the
// Authenticator in.
package auth

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// The cookies the client's browser carries. Both are httpOnly: no script reads them.
const (
	SessionCookie = "session"
	RefreshCookie = "refresh"
)

// Principal is who is calling.
type Principal struct {
	ID    string
	Name  string
	Email string
	Role  string
	// Workspaces are the ids of the workspaces they belong to.
	Workspaces []string
	// SessionEndsAt is when the session the call carried ends.
	SessionEndsAt time.Time
}

// Belongs says whether the person is in the workspace.
func (p *Principal) Belongs(workspace string) bool { return slices.Contains(p.Workspaces, workspace) }

// Scope is what a read covers: the caller's workspaces, or the one asked
// when it is theirs; one that isn't is problem.ErrNotFound, as if it weren't there.
func (p *Principal) Scope(asked string) ([]string, error) {
	if asked == "" {
		return p.Workspaces, nil
	}

	if !p.Belongs(asked) {
		return nil, problem.ErrNotFound
	}

	return []string{asked}, nil
}

// Authenticator finds the person a session token names, or problem.ErrUnauthorized.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (Principal, error)
}

type principalKey struct{}

// From is the caller, when the request carried a live session.
func From(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*Principal)

	return p, ok
}

// With puts a caller on a context: the middleware does it, and tests.
func With(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// Required is the caller, or problem.ErrUnauthorized.
func Required(ctx context.Context) (*Principal, error) {
	if p, ok := From(ctx); ok {
		return p, nil
	}

	return nil, problem.ErrUnauthorized
}

type tokensKey struct{}

// Tokens are the cookies' values a request carried, for the operations that
// end or renew a session; "" when it carried none.
type Tokens struct {
	Session string
	Refresh string
}

// TokensFrom is what the request's cookies carried.
func TokensFrom(ctx context.Context) Tokens {
	t, _ := ctx.Value(tokensKey{}).(Tokens)

	return t
}

// Middleware puts the caller a session cookie names on the request, and the
// cookies' tokens. A request with no cookie, or one that has ended, goes on
// with nobody: each operation says whether it needs someone (Required).
func Middleware(a Authenticator, log Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := Tokens{Session: cookie(r, SessionCookie), Refresh: cookie(r, RefreshCookie)}
			ctx := context.WithValue(r.Context(), tokensKey{}, t)

			if t.Session != "" {
				p, err := a.Authenticate(ctx, t.Session)
				if err == nil {
					ctx = With(ctx, &p)
				} else {
					log.DebugContext(ctx, "a session cookie that names nobody", "err", err)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func cookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}

	return c.Value
}

// Logger is what Middleware logs to (a *slog.Logger).
type Logger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
}

// CSRF refuses a write without X-Requested-With: XMLHttpRequest, which the
// client sends on every one. The session is a cookie, which the browser
// sends from any site; that header it doesn't, unless CORS allows it.
func CSRF(log problem.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
			if !safe && r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				problem.Write(w, r, log, problem.ErrForbidden)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
