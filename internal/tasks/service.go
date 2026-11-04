// Package tasks is a workspace's tasks: a page of those that match, with how
// many of them are late, one, making, changing and deleting one. A task of a
// workspace the caller isn't in is not there for them (404); what a role may
// do is the contract's x-permission, checked before a method runs
// (platform/auth.Rules). A task's project is one of its own workspace's; a
// task is late when its due day is before today (the server's day, in UTC)
// and it isn't done. A file can't be named yet: nothing keeps one (POST
// /files is not built), so any is refused as the client's mock refuses a
// file it never got.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

const (
	// PageSize is a page's rows unless the client asks for another, as its lib/pages.ts does.
	PageSize = 50
	// MaxPage is the most a page may hold: more is refused, never cut short silently.
	MaxPage = 200
)

// Task -.
type Task struct {
	ID        string
	Workspace string
	Title     string
	Status    string
	// Due is the day it is due, at midnight UTC; nil has none.
	Due *time.Time
	// Project and ProjectName are its project's; "" has none.
	Project     string
	ProjectName string
	UpdatedAt   time.Time
}

// Query is what a list asks of the store.
type Query struct {
	Workspaces []string
	Q          string
	Status     string
	HideDone   bool
	Project    string
	// Sort is title, due or updated ("" leaves the store's order: the last changed first).
	Sort       string
	Descending bool
	Offset     int
	Limit      int
	// Today is the day a task due before is late.
	Today time.Time
}

// Counts are what the store says of every match, not only the page's.
type Counts struct {
	Total int
	// Late are past their day and not done.
	Late int
}

// Page is the tasks on one page, and what is true of every match.
type Page struct {
	Items []Task
	Counts
	// Next is where the next page starts; "" on the last.
	Next string
}

// Store is what the service keeps tasks in (store.go, on Postgres).
type Store interface {
	// List is a page of the matches, sorted, and the counts of all of them.
	List(ctx context.Context, q *Query) ([]Task, Counts, error)
	// Get is the task, or problem.ErrNotFound.
	Get(ctx context.Context, id string) (Task, error)
	// ProjectName is the name of the workspace's project, or problem.ErrNotFound.
	ProjectName(ctx context.Context, workspace, id string) (string, error)
	Create(ctx context.Context, t *Task) (Task, error)
	Update(ctx context.Context, t *Task) (Task, error)
	Delete(ctx context.Context, id string) error
}

// Service -.
type Service struct {
	store Store
	now   func() time.Time
}

// New is the service on the clock given (today is now's day, in UTC).
func New(store Store, now func() time.Time) *Service { return &Service{store: store, now: now} }

// Ask is a list's request as the client sends it.
type Ask struct {
	Workspace, Q, Status, Project, Sort, Cursor string
	HideDone                                    bool
	Limit                                       int
}

// List is a page of the caller's tasks that match.
func (s *Service) List(ctx context.Context, a *Ask) (Page, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Page{}, err
	}

	scope, err := workspaces(p, a.Workspace)
	if err != nil {
		return Page{}, err
	}

	q := Query{
		Workspaces: scope, Q: strings.TrimSpace(a.Q), Status: a.Status, HideDone: a.HideDone, Project: a.Project,
		Limit: PageSize, Today: day(s.now()),
	}
	q.Sort, q.Descending = sortOf(a.Sort)
	q.Offset = offsetOf(a.Cursor)

	switch {
	case a.Limit > MaxPage:
		return Page{}, problem.Invalid("limit", fmt.Sprintf("A page holds at most %d.", MaxPage))
	case a.Limit > 0:
		q.Limit = a.Limit
	}

	items, counts, err := s.store.List(ctx, &q)
	if err != nil {
		return Page{}, fmt.Errorf("tasks: list: %w", err)
	}

	page := Page{Items: items, Counts: counts}
	if end := q.Offset + len(items); end < counts.Total {
		page.Next = strconv.Itoa(end)
	}

	return page, nil
}

// Get is one of the caller's tasks.
func (s *Service) Get(ctx context.Context, id string) (Task, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Task{}, err
	}

	return s.mine(ctx, p, id)
}

// Input is what making a task sends; nil and "" are none.
type Input struct {
	Title, Status, Workspace string
	Due                      *time.Time
	Project, Attachment      *string
}

// Create makes a task in one of the caller's workspaces (their first, when none is named), to do
// unless another status is sent.
func (s *Service) Create(ctx context.Context, in *Input) (Task, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Task{}, err
	}

	ws := in.Workspace
	if ws == "" && len(p.Workspaces) > 0 {
		ws = p.Workspaces[0]
	}

	if !p.Belongs(ws) {
		return Task{}, problem.ErrNotFound
	}

	next := Task{Workspace: ws, Status: "todo"}
	if in.Status != "" {
		next.Status = in.Status
	}

	if err := s.apply(ctx, &next, &Patch{Title: &in.Title, Due: in.Due, Project: in.Project, Attachment: in.Attachment}); err != nil {
		return Task{}, err
	}

	made, err := s.store.Create(ctx, &next)
	if err != nil {
		return Task{}, fmt.Errorf("tasks: create: %w", err)
	}

	return made, nil
}

// Patch is what changing a task sends: nil is left as it is, and a No… takes it off (the client
// sent null).
type Patch struct {
	Title, Status       *string
	Due                 *time.Time
	Project, Attachment *string
	NoDue, NoProject    bool
}

// Update changes one of the caller's tasks. Its workspace stays: a task isn't moved.
func (s *Service) Update(ctx context.Context, id string, patch *Patch) (Task, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Task{}, err
	}

	next, err := s.mine(ctx, p, id)
	if err != nil {
		return Task{}, err
	}

	if err := s.apply(ctx, &next, patch); err != nil {
		return Task{}, err
	}

	saved, err := s.store.Update(ctx, &next)
	if err != nil {
		return Task{}, fmt.Errorf("tasks: update: %w", err)
	}

	return saved, nil
}

// Delete removes one of the caller's tasks.
func (s *Service) Delete(ctx context.Context, id string) error {
	p, err := auth.Required(ctx)
	if err != nil {
		return err
	}

	if _, err := s.mine(ctx, p, id); err != nil {
		return err
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("tasks: delete: %w", err)
	}

	return nil
}

// apply puts what a create or a change sends on the task, refusing what the rules don't allow: no
// title, a project of another workspace (or none), a file.
func (s *Service) apply(ctx context.Context, t *Task, patch *Patch) error {
	if patch.Title != nil {
		t.Title = strings.TrimSpace(*patch.Title)
		if t.Title == "" {
			return problem.Invalid("title", "A task needs a title.")
		}
	}

	if patch.Status != nil {
		t.Status = *patch.Status
	}

	switch {
	case patch.NoDue:
		t.Due = nil
	case patch.Due != nil:
		d := day(*patch.Due)
		t.Due = &d
	}

	switch {
	case patch.NoProject:
		t.Project, t.ProjectName = "", ""
	case patch.Project != nil:
		name, err := s.store.ProjectName(ctx, t.Workspace, *patch.Project)
		if errors.Is(err, problem.ErrNotFound) {
			return problem.Invalid("project", "Pick a project in this workspace.")
		}

		if err != nil {
			return fmt.Errorf("tasks: a project: %w", err)
		}

		t.Project, t.ProjectName = *patch.Project, name
	}

	// A task has no file to take off (null), and none can be named yet.
	if patch.Attachment != nil {
		return problem.Invalid("attachment", "That file didn't arrive; add it again.")
	}

	return nil
}

// mine is the task when it is in one of the caller's workspaces; another's is not there.
func (s *Service) mine(ctx context.Context, p *auth.Principal, id string) (Task, error) {
	t, err := s.store.Get(ctx, id)
	if err != nil {
		return Task{}, fmt.Errorf("tasks: %w", err)
	}

	if !p.Belongs(t.Workspace) {
		return Task{}, problem.ErrNotFound
	}

	return t, nil
}

// workspaces is the scope of a read: the caller's, or the one asked when it is theirs.
func workspaces(p *auth.Principal, asked string) ([]string, error) {
	if asked == "" {
		return p.Workspaces, nil
	}

	if !p.Belongs(asked) {
		return nil, problem.ErrNotFound
	}

	return []string{asked}, nil
}

// day is the day of t, in UTC, at midnight.
func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// sortOf reads "title:desc"; a column the list doesn't sort by leaves the store's order.
func sortOf(s string) (string, bool) {
	col, dir, _ := strings.Cut(s, ":")
	if !slices.Contains([]string{"title", "due", "updated"}, col) {
		return "", false
	}

	return col, dir == "desc"
}

// offsetOf reads a cursor (the offset, as the client's lib/pages.ts writes it); one that isn't, or
// one past what a query's OFFSET takes (int32), starts at the top.
func offsetOf(cursor string) int {
	n, err := strconv.ParseInt(cursor, 10, 32)
	if err != nil || n < 0 {
		return 0
	}

	return int(n)
}
