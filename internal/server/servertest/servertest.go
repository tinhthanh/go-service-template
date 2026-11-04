// Package servertest is the real server for a test, on stores in memory: a
// feature's HTTP tests (its package _test) call it through contracttest, so
// every answer is checked against the contract, and sign people in with a
// clock the test moves (never a wait).
package servertest

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/config"
	"github.com/tinhthanh/go-service-template/internal/platform/contracttest"
	"github.com/tinhthanh/go-service-template/internal/server"
	"github.com/tinhthanh/go-service-template/internal/session"
	"github.com/tinhthanh/go-service-template/internal/session/sessiontest"
)

// Prefix is where the test server mounts the contract.
const Prefix = "/api"

// The people every test server has, all with Password.
const (
	Password = "acme-password"
	Owner    = "u-owner"
	Member   = "u-member"
	Viewer   = "u-viewer"
)

// Server is the server and what a test reaches into.
type Server struct {
	*contracttest.Client
	Sessions *sessiontest.Store
	clock    *Clock
	t        *testing.T
}

// Clock is the server's time, which a test moves.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// Now -.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

// Advance moves the clock on.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}

// Lengths are the test server's sessions: an hour, renewable for a day.
var Lengths = session.Lengths{Session: time.Hour, Refresh: 24 * time.Hour} //nolint:gochecknoglobals // a fixed setting tests read

// New is a server with an owner, a member and a viewer of workspace w-acme.
func New(t *testing.T) *Server {
	t.Helper()

	clock := &Clock{now: time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)}
	store := sessiontest.New(clock.Now)

	for _, p := range []auth.Principal{
		{ID: Owner, Name: "Olive Owner", Email: "owner@acme.test", Role: "owner", Workspaces: []string{"w-acme"}},
		{ID: Member, Name: "Max Member", Email: "member@acme.test", Role: "member", Workspaces: []string{"w-acme"}},
		{ID: Viewer, Name: "Vic Viewer", Email: "viewer@acme.test", Role: "viewer", Workspaces: []string{"w-acme"}},
	} {
		store.Add(p, Password)
	}

	sessions, err := session.New(store, Lengths, clock.Now)
	require.NoError(t, err)

	cfg := &config.Config{APIPrefix: Prefix, CORSOrigins: []string{"https://app.acme.test"}, CookieSecure: true}

	h, err := server.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), server.Features{
		Auth:    sessions,
		Session: session.NewHandler(sessions, session.Cookies{Secure: true, RefreshPath: Prefix + "/session/refresh"}),
	})
	require.NoError(t, err)

	return &Server{Client: contracttest.New(t, h, Prefix), Sessions: store, clock: clock, t: t}
}

// Clock is the server's clock.
func (s *Server) Clock() *Clock { return s.clock }

// SignIn signs the person in and answers the Cookie header their browser would send.
func (s *Server) SignIn(email string) string {
	s.t.Helper()

	a := s.Do(http.MethodPost, "/session", `{"email": "`+email+`", "password": "`+Password+`"}`, nil)
	require.Equal(s.t, http.StatusOK, a.Status, string(a.Body))

	return CookieHeader(a.Cookies())
}

// CookieHeader is the Cookie header for the cookies an answer set.
func CookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}

	return strings.Join(parts, "; ")
}
