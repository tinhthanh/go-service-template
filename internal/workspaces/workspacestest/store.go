// Package workspacestest is the workspaces store in memory, for tests that
// call the server without a database. Joining tells the session store, so
// the person's next call belongs to the workspace, as Postgres's would.
package workspacestest

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/tinhthanh/go-service-template/internal/workspaces"
)

// Store -.
type Store struct {
	mu      sync.Mutex
	all     map[string]workspaces.Workspace
	members map[string][]string
	onJoin  func(userID, workspace string)
}

var _ workspaces.Store = (*Store)(nil)

// New is a store whose joins are told to onJoin.
func New(onJoin func(userID, workspace string)) *Store {
	return &Store{all: map[string]workspaces.Workspace{}, members: map[string][]string{}, onJoin: onJoin}
}

// Add is a workspace with these people in it (the session store already has them).
func (s *Store) Add(w workspaces.Workspace, members ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.all[w.ID] = w
	for _, m := range members {
		s.members[m] = append(s.members[m], w.ID)
	}
}

// Of -.
func (s *Store) Of(_ context.Context, userID string) ([]workspaces.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := []workspaces.Workspace{}
	for _, id := range s.members[userID] {
		out = append(out, s.all[id])
	}

	slices.SortFunc(out, func(a, b workspaces.Workspace) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})

	return out, nil
}

// Taken -.
func (s *Store) Taken(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.all[id]

	return ok, nil
}

// Create -.
func (s *Store) Create(_ context.Context, w workspaces.Workspace, member string) error {
	s.mu.Lock()
	s.all[w.ID] = w
	s.members[member] = append(s.members[member], w.ID)
	s.mu.Unlock()

	s.onJoin(member, w.ID)

	return nil
}
