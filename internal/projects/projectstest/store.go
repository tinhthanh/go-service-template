// Package projectstest is the projects store in memory, for tests that call
// the server without a database. It filters and sorts as the queries in
// queries.sql do; the store's own tests prove those on Postgres.
package projectstest

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/projects"
)

// Store -.
type Store struct {
	mu   sync.Mutex
	now  func() time.Time
	rows []projects.Project
	n    int
}

var _ projects.Store = (*Store)(nil)

// New is an empty store on the clock given.
func New(now func() time.Time) *Store { return &Store{now: now} }

// Add is a project as it is, its id kept.
func (s *Store) Add(p *projects.Project) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.Status == "" {
		p.Status = "active"
	}

	if p.Tags == nil {
		p.Tags = []string{}
	}

	s.rows = append(s.rows, *p)
}

// List -.
func (s *Store) List(_ context.Context, q *projects.Query) ([]projects.Project, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	match := []projects.Project{}

	for i := range s.rows {
		if matches(&s.rows[i], q) {
			match = append(match, s.rows[i])
		}
	}

	slices.SortStableFunc(match, order(q))

	start := min(q.Offset, len(match))
	end := min(start+q.Limit, len(match))

	return slices.Clone(match[start:end]), len(match), nil
}

func matches(p *projects.Project, q *projects.Query) bool {
	words := strings.ToLower(p.Name + " " + p.Description + " " + p.Owner)

	return slices.Contains(q.Workspaces, p.Workspace) &&
		(q.Q == "" || strings.Contains(words, strings.ToLower(q.Q))) &&
		(q.Status == "" || p.Status == q.Status) &&
		!slices.ContainsFunc(q.Tags, func(t string) bool { return !slices.Contains(p.Tags, t) })
}

// order is the queries' ORDER BY: the column asked, then the last changed first, then the id.
func order(q *projects.Query) func(a, b projects.Project) int {
	return func(a, b projects.Project) int {
		var c int

		switch q.Sort {
		case "name":
			c = cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case "owner":
			c = cmp.Compare(strings.ToLower(a.Owner), strings.ToLower(b.Owner))
		case "updated":
			c = a.UpdatedAt.Compare(b.UpdatedAt)
		}

		if q.Descending {
			c = -c
		}

		return cmp.Or(c, b.UpdatedAt.Compare(a.UpdatedAt), cmp.Compare(a.ID, b.ID))
	}
}

// Get -.
func (s *Store) Get(_ context.Context, id string) (projects.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i := s.index(id); i >= 0 {
		return s.rows[i], nil
	}

	return projects.Project{}, problem.ErrNotFound
}

// NameTaken -.
func (s *Store) NameTaken(_ context.Context, workspace, name, otherThan string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.ContainsFunc(s.rows, func(p projects.Project) bool {
		return p.Workspace == workspace && strings.EqualFold(p.Name, name) && p.ID != otherThan
	}), nil
}

// Create -.
func (s *Store) Create(_ context.Context, p *projects.Project) (projects.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.n++
	made := *p
	made.ID = "p-" + strconv.Itoa(s.n)
	made.Status = "active"
	made.Tags = []string{}
	made.UpdatedAt = s.now()
	s.rows = append(s.rows, made)

	return made, nil
}

// Update -.
func (s *Store) Update(_ context.Context, p *projects.Project) (projects.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	i := s.index(p.ID)
	if i < 0 {
		return projects.Project{}, problem.ErrNotFound
	}

	s.rows[i] = *p
	s.rows[i].UpdatedAt = s.now()

	return s.rows[i], nil
}

// Delete -.
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rows = slices.DeleteFunc(s.rows, func(p projects.Project) bool { return p.ID == id })

	return nil
}

// Tags -.
func (s *Store) Tags(_ context.Context, workspaces []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tags := []string{}

	for i := range s.rows {
		if slices.Contains(workspaces, s.rows[i].Workspace) {
			tags = append(tags, s.rows[i].Tags...)
		}
	}

	slices.Sort(tags)

	return slices.Compact(tags), nil
}

func (s *Store) index(id string) int {
	return slices.IndexFunc(s.rows, func(p projects.Project) bool { return p.ID == id })
}
