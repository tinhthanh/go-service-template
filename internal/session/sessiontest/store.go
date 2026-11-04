// Package sessiontest is the session store in memory, for tests that call
// the server without a database: people are added with Add, and sessions
// end when the test's clock passes them, as Postgres's would.
package sessiontest

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/session"
)

// Store -.
type Store struct {
	mu       sync.Mutex
	now      func() time.Time
	people   map[string]person
	sessions map[string]stored
}

type person struct {
	auth.Principal
	hash string
}

type stored struct {
	userID string
	pair   session.Pair
}

var _ session.Store = (*Store)(nil)

// New is an empty store on the clock given.
func New(now func() time.Time) *Store {
	return &Store{now: now, people: map[string]person{}, sessions: map[string]stored{}}
}

// Add is a person who signs in with this password.
func (s *Store) Add(p auth.Principal, password string) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.people[p.ID] = person{Principal: p, hash: string(hash)}
}

// Sessions is how many sessions are kept.
func (s *Store) Sessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.sessions)
}

// Account -.
func (s *Store) Account(_ context.Context, email string) (session.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id := range s.people {
		if p := s.people[id]; strings.EqualFold(p.Email, email) {
			return session.Account{ID: p.ID, PasswordHash: p.hash}, nil
		}
	}

	return session.Account{}, problem.ErrNotFound
}

// Create -.
func (s *Store) Create(_ context.Context, userID string, p session.Pair) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[string(p.TokenHash)] = stored{userID: userID, pair: p}

	return nil
}

// Person -.
func (s *Store) Person(_ context.Context, tokenHash []byte) (auth.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.sessions[string(tokenHash)]
	if !ok || !st.pair.EndsAt.After(s.now()) {
		return auth.Principal{}, problem.ErrNotFound
	}

	p := s.people[st.userID].Principal
	p.Workspaces = slices.Clone(p.Workspaces)
	p.SessionEndsAt = st.pair.EndsAt

	return p, nil
}

// Renew -.
func (s *Store) Renew(_ context.Context, refreshHash []byte, next session.Pair) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, st := range s.sessions {
		if bytes.Equal(st.pair.RefreshHash, refreshHash) && st.pair.RefreshEndsAt.After(s.now()) {
			delete(s.sessions, k)
			s.sessions[string(next.TokenHash)] = stored{userID: st.userID, pair: next}

			return nil
		}
	}

	return problem.ErrNotFound
}

// Delete -.
func (s *Store) Delete(_ context.Context, tokenHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, string(tokenHash))

	return nil
}
