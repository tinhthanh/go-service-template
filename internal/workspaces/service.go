// Package workspaces is the workspaces a person belongs to, and making one:
// its id is made from its name (w-acme, w-acme-2…), as the client's mock
// makes it, and its maker belongs to it.
package workspaces

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// Workspace -.
type Workspace struct {
	ID   string
	Name string
}

// Store is what the service keeps workspaces in (store.go, on Postgres).
type Store interface {
	// Of is the workspaces the person belongs to, by name.
	Of(ctx context.Context, userID string) ([]Workspace, error)
	Taken(ctx context.Context, id string) (bool, error)
	// Create makes the workspace with the person in it, at once or not at all.
	Create(ctx context.Context, w Workspace, member string) error
}

// Service -.
type Service struct {
	store Store
}

// New -.
func New(store Store) *Service { return &Service{store: store} }

// List is the caller's workspaces.
func (s *Service) List(ctx context.Context) ([]Workspace, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return nil, err
	}

	ws, err := s.store.Of(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("workspaces: list: %w", err)
	}

	return ws, nil
}

// Create makes a workspace the caller belongs to. A name is needed, and
// one of the caller's own workspaces may not have it already.
func (s *Service) Create(ctx context.Context, name string) (Workspace, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Workspace{}, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, problem.Invalid("name", "A workspace needs a name.")
	}

	mine, err := s.store.Of(ctx, p.ID)
	if err != nil {
		return Workspace{}, fmt.Errorf("workspaces: create: %w", err)
	}

	for _, w := range mine {
		if strings.EqualFold(w.Name, name) {
			return Workspace{}, problem.Invalid("name", "You're already in a workspace called "+w.Name+".")
		}
	}

	id, err := s.freeID(ctx, slug(name))
	if err != nil {
		return Workspace{}, err
	}

	w := Workspace{ID: id, Name: name}
	if err := s.store.Create(ctx, w, p.ID); err != nil {
		return Workspace{}, fmt.Errorf("workspaces: create: %w", err)
	}

	return w, nil
}

// freeID is w-<slug>, or w-<slug>-2, -3… when it is taken.
func (s *Service) freeID(ctx context.Context, base string) (string, error) {
	id := "w-" + base

	for n := 2; ; n++ {
		taken, err := s.store.Taken(ctx, id)
		if err != nil {
			return "", fmt.Errorf("workspaces: an id: %w", err)
		}

		if !taken {
			return id, nil
		}

		id = "w-" + base + "-" + strconv.Itoa(n)
	}
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug is the name in lowercase letters, digits and dashes; "workspace" for a name with none.
func slug(name string) string {
	s := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if s == "" {
		return "workspace"
	}

	return s
}
