// Package servertest is the real server for a test, on stores in memory: a
// feature's HTTP tests (its package _test) call it through contracttest, so
// every answer is checked against the contract, and sign people in with a
// clock the test moves (never a wait).
package servertest

import (
	"context"
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
	"github.com/tinhthanh/go-service-template/internal/projects"
	"github.com/tinhthanh/go-service-template/internal/projects/projectstest"
	"github.com/tinhthanh/go-service-template/internal/search"
	"github.com/tinhthanh/go-service-template/internal/search/searchtest"
	"github.com/tinhthanh/go-service-template/internal/server"
	"github.com/tinhthanh/go-service-template/internal/session"
	"github.com/tinhthanh/go-service-template/internal/session/sessiontest"
	"github.com/tinhthanh/go-service-template/internal/tasks"
	"github.com/tinhthanh/go-service-template/internal/tasks/taskstest"
	"github.com/tinhthanh/go-service-template/internal/workspaces"
	"github.com/tinhthanh/go-service-template/internal/workspaces/workspacestest"
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
	Sessions   *sessiontest.Store
	Workspaces *workspacestest.Store
	Projects   *projectstest.Store
	Tasks      *taskstest.Store
	clock      *Clock
	t          *testing.T
}

// The workspaces every test server has: the three people are in Acme;
// Initrode is somebody else's, with Vault in it, and Vault's task Safe.
const (
	Acme     = "w-acme"
	Initrode = "w-initrode"
	Vault    = "p-vault"
	Safe     = "t-safe"
)

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
// As signs a call in as one of them: s.Do(…, s.As(servertest.Member)).
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

	ws := workspacestest.New(store.Join)
	ws.Add(workspaces.Workspace{ID: Acme, Name: "Acme"}, Owner, Member, Viewer)
	ws.Add(workspaces.Workspace{ID: Initrode, Name: "Initrode"})

	prs := projectstest.New(clock.Now)
	prs.Add(&projects.Project{ID: Vault, Workspace: Initrode, Name: "Vault", Description: "Kept by another team.", Owner: "Kim", UpdatedAt: clock.Now()})

	// A task's project is read from the projects store, as the tasks queries JOIN projects.
	tks := taskstest.New(clock.Now, func(ctx context.Context, id string) (string, string, error) {
		p, err := prs.Get(ctx, id)

		return p.Workspace, p.Name, err
	})
	tks.Add(&tasks.Task{ID: Safe, Workspace: Initrode, Title: "Open the safe", Project: Vault, UpdatedAt: clock.Now()})

	cfg := &config.Config{APIPrefix: Prefix, CORSOrigins: []string{"https://app.acme.test"}, CookieSecure: true}

	h, err := server.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), server.Features{
		Auth:       sessions,
		Session:    session.NewHandler(sessions, session.Cookies{Secure: true, RefreshPath: Prefix + "/session/refresh"}),
		Workspaces: workspaces.NewHandler(workspaces.New(ws)),
		Projects:   projects.NewHandler(projects.New(prs)),
		Tasks:      tasks.NewHandler(tasks.New(tks, clock.Now)),
		Search:     search.NewHandler(search.New(searchtest.New(func() []searchtest.Row { return found(prs, tks) }))),
	})
	require.NoError(t, err)

	return &Server{Client: contracttest.New(t, h, Prefix), Sessions: store, Workspaces: ws, Projects: prs, Tasks: tks, clock: clock, t: t}
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

// As is the headers of a call by the person (Owner, Member, Viewer): their session's cookie.
func (s *Server) As(who string) map[string]string {
	s.t.Helper()

	email := map[string]string{Owner: "owner@acme.test", Member: "member@acme.test", Viewer: "viewer@acme.test"}[who]

	return map[string]string{"Cookie": s.SignIn(email)}
}

// found is every project and task, as search's store in memory reads them: a task's detail is its
// project's name as it is now.
func found(prs *projectstest.Store, tks *taskstest.Store) []searchtest.Row {
	names := map[string]string{}
	rows := []searchtest.Row{}

	all := prs.All()
	for i := range all {
		p := &all[i]
		names[p.ID] = p.Name
		rows = append(rows, searchtest.Row{
			Kind: "project", ID: p.ID, Name: p.Name, Detail: p.Owner, Status: p.Status,
			Text: p.Description + " " + p.Owner, Workspace: p.Workspace,
		})
	}

	ts := tks.All()
	for i := range ts {
		t := &ts[i]
		rows = append(rows, searchtest.Row{
			Kind: "task", ID: t.ID, Name: t.Title, Detail: names[t.Project], Status: t.Status,
			Text: names[t.Project], Workspace: t.Workspace,
		})
	}

	return rows
}
