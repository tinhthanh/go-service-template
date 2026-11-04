package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tinhthanh/go-service-template/internal/api"
	"github.com/tinhthanh/go-service-template/internal/platform/auth"
)

// Cookies are how the session's cookies are set.
type Cookies struct {
	// Secure sends them over HTTPS only: off only for plain HTTP on a laptop.
	Secure bool
	// RefreshPath is the only path the refresh cookie goes to ("/api/session/refresh").
	RefreshPath string
}

// Handler answers the contract's session operations: me, signIn, signOut, refreshSession.
type Handler struct {
	svc     *Service
	cookies Cookies
}

// NewHandler -.
func NewHandler(svc *Service, cookies Cookies) *Handler { return &Handler{svc: svc, cookies: cookies} }

// Me is who is signed in, or 401.
func (h *Handler) Me(ctx context.Context, _ api.MeRequestObject) (api.MeResponseObject, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return nil, err
	}

	return api.Me200JSONResponse(user(p)), nil
}

// SignIn starts a session and sets its cookies.
func (h *Handler) SignIn(ctx context.Context, req api.SignInRequestObject) (api.SignInResponseObject, error) {
	tokens, p, err := h.svc.SignIn(ctx, req.Body.Email, req.Body.Password)
	if err != nil {
		return nil, err
	}

	return withCookies{h: h, tokens: tokens, body: user(&p)}, nil
}

// RefreshSession renews the session the refresh cookie names, with new cookies.
func (h *Handler) RefreshSession(ctx context.Context, _ api.RefreshSessionRequestObject) (api.RefreshSessionResponseObject, error) {
	tokens, p, err := h.svc.Refresh(ctx, auth.TokensFrom(ctx).Refresh)
	if err != nil {
		return nil, err
	}

	return withCookies{h: h, tokens: tokens, body: user(&p)}, nil
}

// SignOut ends the session and its cookies; signed out already is no error.
func (h *Handler) SignOut(ctx context.Context, _ api.SignOutRequestObject) (api.SignOutResponseObject, error) {
	if err := h.svc.SignOut(ctx, auth.TokensFrom(ctx).Session); err != nil {
		return nil, err
	}

	return withCookies{h: h, ended: true}, nil
}

func user(p *auth.Principal) api.User {
	u := api.User{Id: p.ID, Name: p.Name, Email: p.Email, Role: api.Role(p.Role)}
	if !p.SessionEndsAt.IsZero() {
		ends := p.SessionEndsAt.UTC()
		u.SessionEndsAt = &ends
	}

	return u
}

// withCookies is an answer that sets the session's cookies (or ends them)
// before its body: the generated answers can't, having no headers in the contract.
type withCookies struct {
	h      *Handler
	tokens Tokens
	body   api.User
	ended  bool
}

func (a withCookies) write(w http.ResponseWriter) error {
	if a.ended {
		gone := time.Unix(0, 0)
		a.h.set(w, auth.SessionCookie, "", "/", gone)
		a.h.set(w, auth.RefreshCookie, "", a.h.cookies.RefreshPath, gone)
		w.WriteHeader(http.StatusNoContent)

		return nil
	}

	a.h.set(w, auth.SessionCookie, a.tokens.Session, "/", a.tokens.EndsAt)
	a.h.set(w, auth.RefreshCookie, a.tokens.Refresh, a.h.cookies.RefreshPath, a.tokens.RefreshEndsAt)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(a.body); err != nil {
		return fmt.Errorf("session: an answer: %w", err)
	}

	return nil
}

// VisitSignInResponse -.
func (a withCookies) VisitSignInResponse(w http.ResponseWriter) error { return a.write(w) }

// VisitRefreshSessionResponse -.
func (a withCookies) VisitRefreshSessionResponse(w http.ResponseWriter) error { return a.write(w) }

// VisitSignOutResponse -.
func (a withCookies) VisitSignOutResponse(w http.ResponseWriter) error { return a.write(w) }

func (h *Handler) set(w http.ResponseWriter, name, value, path string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is COOKIE_SECURE: on unless plain HTTP on a laptop
		Name: name, Value: value, Path: path, Expires: expires,
		HttpOnly: true, Secure: h.cookies.Secure, SameSite: http.SameSiteLaxMode,
	})
}
