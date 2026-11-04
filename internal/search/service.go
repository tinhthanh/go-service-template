// Package search is what a few letters find among the caller's records:
// projects and tasks, each kind ranked on its own (PerKind at most), so one
// can't crowd the other out, as the client's lib/search-rank.ts ranks them.
// It reads the projects and tasks tables to find them and never writes them.
package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
)

// PerKind is the most of each kind a search answers, as the client's rankEach does.
const PerKind = 5

// Hit is one record found.
type Hit struct {
	Kind, ID, Name, Detail, Status string
}

// Store is what the service finds records in (store.go, on Postgres).
type Store interface {
	// Find is the projects, then the tasks, that match q in the workspaces, best first, per of each.
	Find(ctx context.Context, q string, workspaces []string, per int) ([]Hit, error)
}

// Service -.
type Service struct {
	store Store
}

// New -.
func New(store Store) *Service { return &Service{store: store} }

// Find is what q finds among the caller's records; nothing for no letters.
func (s *Service) Find(ctx context.Context, q string) ([]Hit, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return nil, err
	}

	if q = strings.TrimSpace(q); q == "" || len(p.Workspaces) == 0 {
		return []Hit{}, nil
	}

	hits, err := s.store.Find(ctx, q, p.Workspaces, PerKind)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	return hits, nil
}
