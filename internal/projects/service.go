// Package projects is a workspace's projects: a page of those that match,
// one, making, changing and deleting one, and the tags in use. A project of
// a workspace the caller isn't in is not there for them (404); what a role
// may do is the contract's x-permission, checked before a method runs
// (platform/auth.Rules). A name is its workspace's own, in any case.
package projects

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/page"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// Project -.
type Project struct {
	ID          string
	Workspace   string
	Name        string
	Description string
	Status      string
	Owner       string
	Tags        []string
	UpdatedAt   time.Time
}

// Query is what a list asks of the store.
type Query struct {
	Workspaces []string
	Q          string
	Status     string
	Tags       []string
	// Sort is name, owner or updated ("" leaves the store's order: the last changed first).
	Sort       string
	Descending bool
	Offset     int
	Limit      int
}

// Page is the projects on one page, and how many match in all.
type Page struct {
	Items []Project
	Total int
	// Next is where the next page starts; "" on the last.
	Next string
}

// Store is what the service keeps projects in (store.go, on Postgres).
type Store interface {
	// List is a page of the matches, sorted, and their total.
	List(ctx context.Context, q *Query) ([]Project, int, error)
	// Get is the project, or problem.ErrNotFound.
	Get(ctx context.Context, id string) (Project, error)
	// NameTaken says whether another project of the workspace has the name, in any case.
	NameTaken(ctx context.Context, workspace, name, otherThan string) (bool, error)
	Create(ctx context.Context, p *Project) (Project, error)
	Update(ctx context.Context, p *Project) (Project, error)
	Delete(ctx context.Context, id string) error
	Tags(ctx context.Context, workspaces []string) ([]string, error)
}

// Service -.
type Service struct {
	store Store
}

// New -.
func New(store Store) *Service { return &Service{store: store} }

// Ask is a list's request as the client sends it.
type Ask struct {
	Workspace, Q, Status, Tags, Sort, Cursor string
	Limit                                    int
}

// List is a page of the caller's projects that match.
func (s *Service) List(ctx context.Context, a *Ask) (Page, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Page{}, err
	}

	scope, err := p.Scope(a.Workspace)
	if err != nil {
		return Page{}, err
	}

	q := Query{Workspaces: scope, Q: strings.TrimSpace(a.Q), Status: a.Status, Tags: tagsOf(a.Tags)}
	q.Sort, q.Descending = sortOf(a.Sort)
	q.Offset = page.Offset(a.Cursor)

	if q.Limit, err = page.Limit(a.Limit); err != nil {
		return Page{}, err
	}

	items, total, err := s.store.List(ctx, &q)
	if err != nil {
		return Page{}, fmt.Errorf("projects: list: %w", err)
	}

	return Page{Items: items, Total: total, Next: page.Next(q.Offset, len(items), total)}, nil
}

// Get is one of the caller's projects.
func (s *Service) Get(ctx context.Context, id string) (Project, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Project{}, err
	}

	return s.mine(ctx, p, id)
}

// Input is what making a project sends.
type Input struct {
	Name, Description, Workspace string
}

// Create makes a project in one of the caller's workspaces (their first, when none is named).
func (s *Service) Create(ctx context.Context, in Input) (Project, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Project{}, err
	}

	ws := in.Workspace
	if ws == "" && len(p.Workspaces) > 0 {
		ws = p.Workspaces[0]
	}

	if !p.Belongs(ws) {
		return Project{}, problem.ErrNotFound
	}

	next := Project{Workspace: ws, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), Owner: p.Name}
	if err := s.nameFree(ctx, &next); err != nil {
		return Project{}, err
	}

	made, err := s.store.Create(ctx, &next)
	if err != nil {
		return Project{}, fmt.Errorf("projects: create: %w", err)
	}

	return made, nil
}

// Patch is what changing a project sends; nil is left as it is.
type Patch struct {
	Name, Description, Status *string
	Tags                      *[]string
}

// Update changes one of the caller's projects.
func (s *Service) Update(ctx context.Context, id string, patch Patch) (Project, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Project{}, err
	}

	next, err := s.mine(ctx, p, id)
	if err != nil {
		return Project{}, err
	}

	if patch.Name != nil {
		next.Name = strings.TrimSpace(*patch.Name)
		if err := s.nameFree(ctx, &next); err != nil {
			return Project{}, err
		}
	}

	if patch.Description != nil {
		next.Description = strings.TrimSpace(*patch.Description)
	}

	if patch.Status != nil {
		next.Status = *patch.Status
	}

	if patch.Tags != nil {
		next.Tags = tidyTags(*patch.Tags)
	}

	saved, err := s.store.Update(ctx, &next)
	if err != nil {
		return Project{}, fmt.Errorf("projects: update: %w", err)
	}

	return saved, nil
}

// Delete removes one of the caller's projects.
func (s *Service) Delete(ctx context.Context, id string) error {
	p, err := auth.Required(ctx)
	if err != nil {
		return err
	}

	if _, err := s.mine(ctx, p, id); err != nil {
		return err
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("projects: delete: %w", err)
	}

	return nil
}

// Tags is every tag in use in the caller's workspaces, or the one asked.
func (s *Service) Tags(ctx context.Context, workspace string) ([]string, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return nil, err
	}

	scope, err := p.Scope(workspace)
	if err != nil {
		return nil, err
	}

	tags, err := s.store.Tags(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("projects: tags: %w", err)
	}

	return tags, nil
}

// mine is the project when it is in one of the caller's workspaces; another's is not there.
func (s *Service) mine(ctx context.Context, p *auth.Principal, id string) (Project, error) {
	pr, err := s.store.Get(ctx, id)
	if err != nil {
		return Project{}, fmt.Errorf("projects: %w", err)
	}

	if !p.Belongs(pr.Workspace) {
		return Project{}, problem.ErrNotFound
	}

	return pr, nil
}

// nameFree refuses an empty name, or one another project of the workspace has.
func (s *Service) nameFree(ctx context.Context, p *Project) error {
	if p.Name == "" {
		return problem.Invalid("name", "A project needs a name.")
	}

	taken, err := s.store.NameTaken(ctx, p.Workspace, p.Name, p.ID)
	if err != nil {
		return fmt.Errorf("projects: a name: %w", err)
	}

	if taken {
		return problem.Invalid("name", "Another project is already called "+p.Name+".")
	}

	return nil
}

// sortOf reads "name:desc"; a column the list doesn't sort by leaves the store's order.
func sortOf(s string) (string, bool) {
	col, dir, _ := strings.Cut(s, ":")
	if !slices.Contains([]string{"name", "owner", "updated"}, col) {
		return "", false
	}

	return col, dir == "desc"
}

// tagsOf reads "a,b": every tag a project must wear.
func tagsOf(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}

	return tidyTags(strings.Split(s, ","))
}

// tidyTags trims each tag and drops the empty and the repeated, in order.
func tidyTags(tags []string) []string {
	out := []string{}

	for _, t := range tags {
		if t = strings.TrimSpace(t); t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}

	return out
}
