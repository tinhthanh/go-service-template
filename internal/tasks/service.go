// Package tasks is a workspace's tasks: a page of those that match, with how
// many of them are late, one, making, changing and deleting one. A task of a
// workspace the caller isn't in is not there for them (404); what a role may
// do is the contract's x-permission, checked before a method runs
// (platform/auth.Rules). A task's project is one of its own workspace's; its
// file is one its maker sent (internal/files), refused otherwise as the
// client's mock refuses a file it never got. A task is late when its due day
// is before today (the server's day, in UTC) and it isn't done.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/page"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
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
	// Attachment is its file; nil has none.
	Attachment *Attachment
	UpdatedAt  time.Time
}

// Attachment is a file a task names.
type Attachment struct {
	ID, Name, Type string
	Size           int
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
	// AttachmentOf is the file when owner sent it, or problem.ErrNotFound.
	AttachmentOf(ctx context.Context, id, owner string) (Attachment, error)
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

	scope, err := p.Scope(a.Workspace)
	if err != nil {
		return Page{}, err
	}

	q := Query{
		Workspaces: scope, Q: strings.TrimSpace(a.Q), Status: a.Status, HideDone: a.HideDone, Project: a.Project,
		Today: day(s.now()),
	}
	q.Sort, q.Descending = sortOf(a.Sort)
	q.Offset = page.Offset(a.Cursor)

	if q.Limit, err = page.Limit(a.Limit); err != nil {
		return Page{}, err
	}

	items, counts, err := s.store.List(ctx, &q)
	if err != nil {
		return Page{}, fmt.Errorf("tasks: list: %w", err)
	}

	return Page{Items: items, Counts: counts, Next: page.Next(q.Offset, len(items), counts.Total)}, nil
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

	if err := s.apply(ctx, p, &next, &Patch{Title: &in.Title, Due: in.Due, Project: in.Project, Attachment: in.Attachment}); err != nil {
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
	Title, Status                  *string
	Due                            *time.Time
	Project, Attachment            *string
	NoDue, NoProject, NoAttachment bool
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

	if err := s.apply(ctx, p, &next, patch); err != nil {
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
// title, a project of another workspace (or none), a file the caller didn't send.
func (s *Service) apply(ctx context.Context, p *auth.Principal, t *Task, patch *Patch) error {
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

	return s.attach(ctx, p, t, patch)
}

// attach takes the task's file off (null), or names one the caller sent; the one it has already stays its own.
func (s *Service) attach(ctx context.Context, p *auth.Principal, t *Task, patch *Patch) error {
	switch {
	case patch.NoAttachment:
		t.Attachment = nil
	case patch.Attachment != nil && (t.Attachment == nil || t.Attachment.ID != *patch.Attachment):
		a, err := s.store.AttachmentOf(ctx, *patch.Attachment, p.ID)
		if errors.Is(err, problem.ErrNotFound) {
			return problem.Invalid("attachment", "That file didn't arrive; add it again.")
		}

		if err != nil {
			return fmt.Errorf("tasks: a file: %w", err)
		}

		t.Attachment = &a
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
