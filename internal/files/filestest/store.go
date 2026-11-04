// Package filestest is the files store in memory, for tests that call the
// server without a database. Owner tells another store (tasks') whose a
// file is, as tasks' query reads the files table.
package filestest

import (
	"context"
	"strconv"
	"sync"

	"github.com/tinhthanh/go-service-template/internal/files"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// Store -.
type Store struct {
	mu    sync.Mutex
	kept  map[string]kept
	count int
}

type kept struct {
	owner string
	files.Attachment
}

var _ files.Store = (*Store)(nil)

// New is an empty store.
func New() *Store { return &Store{kept: map[string]kept{}} }

// Create -.
func (s *Store) Create(_ context.Context, owner string, a *files.Attachment, _ []byte) (files.Attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count++
	made := *a
	made.ID = "f-" + strconv.Itoa(s.count)
	s.kept[made.ID] = kept{owner: owner, Attachment: made}

	return made, nil
}

// Of is the file with the id when the owner sent it, or problem.ErrNotFound.
func (s *Store) Of(id, owner string) (files.Attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if k, ok := s.kept[id]; ok && k.owner == owner {
		return k.Attachment, nil
	}

	return files.Attachment{}, problem.ErrNotFound
}
