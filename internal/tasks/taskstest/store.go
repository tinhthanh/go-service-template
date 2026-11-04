// Package taskstest is the tasks store in memory, for tests that call the
// server without a database. It filters, sorts and counts as the queries in
// queries.sql do, and reads a task's project from Projects as their JOIN
// does; the store's own tests prove those on Postgres.
package taskstest

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/tasks"
)

// Projects is the projects a task may name: the workspace and name of one, or problem.ErrNotFound.
type Projects func(ctx context.Context, id string) (workspace, name string, err error)

// Store -.
type Store struct {
	mu       sync.Mutex
	now      func() time.Time
	projects Projects
	rows     []tasks.Task
	n        int
}

var _ tasks.Store = (*Store)(nil)

// New is an empty store on the clock given, its tasks' projects read from projects.
func New(now func() time.Time, projects Projects) *Store {
	return &Store{now: now, projects: projects}
}

// Add is a task as it is, its id kept.
func (s *Store) Add(t *tasks.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.Status == "" {
		t.Status = "todo"
	}

	s.rows = append(s.rows, *t)
}

// List -.
func (s *Store) List(ctx context.Context, q *tasks.Query) ([]tasks.Task, tasks.Counts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	match := []tasks.Task{}
	counts := tasks.Counts{}

	for i := range s.rows {
		t := s.read(ctx, &s.rows[i])
		if !matches(&t, q) {
			continue
		}

		match = append(match, t)
		counts.Total++

		if t.Due != nil && t.Due.Before(q.Today) && t.Status != "done" {
			counts.Late++
		}
	}

	slices.SortStableFunc(match, order(q))

	start := min(q.Offset, len(match))
	end := min(start+q.Limit, len(match))

	return slices.Clone(match[start:end]), counts, nil
}

func matches(t *tasks.Task, q *tasks.Query) bool {
	words := strings.ToLower(t.Title + " " + t.ProjectName)

	return slices.Contains(q.Workspaces, t.Workspace) &&
		(q.Q == "" || strings.Contains(words, strings.ToLower(q.Q))) &&
		(q.Status == "" || t.Status == q.Status) &&
		(!q.HideDone || t.Status != "done") &&
		(q.Project == "" || t.Project == q.Project)
}

// order is the queries' ORDER BY: the column asked (no due day last, or first when descending),
// then the last changed first, then the id.
func order(q *tasks.Query) func(a, b tasks.Task) int {
	return func(a, b tasks.Task) int {
		var c int

		switch q.Sort {
		case "title":
			c = cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		case "due":
			c = compareDue(a.Due, b.Due)
		case "updated":
			c = a.UpdatedAt.Compare(b.UpdatedAt)
		}

		if q.Descending {
			c = -c
		}

		return cmp.Or(c, b.UpdatedAt.Compare(a.UpdatedAt), cmp.Compare(a.ID, b.ID))
	}
}

// compareDue puts no day after every day, as NULLS LAST does.
func compareDue(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}

	return a.Compare(*b)
}

// Get -.
func (s *Store) Get(ctx context.Context, id string) (tasks.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i := s.index(id); i >= 0 {
		return s.read(ctx, &s.rows[i]), nil
	}

	return tasks.Task{}, problem.ErrNotFound
}

// ProjectName -.
func (s *Store) ProjectName(ctx context.Context, workspace, id string) (string, error) {
	ws, name, err := s.projects(ctx, id)
	if err != nil {
		return "", err
	}

	if ws != workspace {
		return "", problem.ErrNotFound
	}

	return name, nil
}

// Create -.
func (s *Store) Create(ctx context.Context, t *tasks.Task) (tasks.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.n++
	made := *t
	made.ID = "t-" + strconv.Itoa(s.n)
	made.UpdatedAt = s.now()
	s.rows = append(s.rows, made)

	return s.read(ctx, &made), nil
}

// Update -.
func (s *Store) Update(ctx context.Context, t *tasks.Task) (tasks.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	i := s.index(t.ID)
	if i < 0 {
		return tasks.Task{}, problem.ErrNotFound
	}

	s.rows[i] = *t
	s.rows[i].UpdatedAt = s.now()

	return s.read(ctx, &s.rows[i]), nil
}

// Delete -.
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rows = slices.DeleteFunc(s.rows, func(t tasks.Task) bool { return t.ID == id })

	return nil
}

// read is the task with its project as it is now, as the JOIN reads it: a project deleted is none.
func (s *Store) read(ctx context.Context, t *tasks.Task) tasks.Task {
	out := *t
	out.ProjectName = ""

	if out.Project == "" {
		return out
	}

	if _, name, err := s.projects(ctx, out.Project); err == nil {
		out.ProjectName = name
	} else {
		out.Project = ""
	}

	return out
}

func (s *Store) index(id string) int {
	return slices.IndexFunc(s.rows, func(t tasks.Task) bool { return t.ID == id })
}

// All is every task kept, for a test's other stores (search) to read.
func (s *Store) All() []tasks.Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.rows)
}
