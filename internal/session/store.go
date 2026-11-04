package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/session/db"
)

// PGStore keeps sessions in Postgres, through the queries sqlc made of queries.sql.
type PGStore struct {
	q *db.Queries
}

// NewPGStore -.
func NewPGStore(conn db.DBTX) *PGStore { return &PGStore{q: db.New(conn)} }

// Account -.
func (s *PGStore) Account(ctx context.Context, email string) (Account, error) {
	row, err := s.q.UserByEmail(ctx, email)
	if err != nil {
		return Account{}, notFound(err)
	}

	return Account{ID: row.ID, PasswordHash: row.PasswordHash}, nil
}

// Create -.
func (s *PGStore) Create(ctx context.Context, userID string, p Pair) error {
	err := s.q.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: p.TokenHash, RefreshHash: p.RefreshHash, UserID: userID, EndsAt: p.EndsAt, RefreshEndsAt: p.RefreshEndsAt,
	})
	if err != nil {
		return fmt.Errorf("session store: create: %w", err)
	}

	return nil
}

// Person -.
func (s *PGStore) Person(ctx context.Context, tokenHash []byte) (auth.Principal, error) {
	row, err := s.q.SessionPerson(ctx, tokenHash)
	if err != nil {
		return auth.Principal{}, notFound(err)
	}

	return auth.Principal{ID: row.ID, Name: row.Name, Email: row.Email, Role: row.Role, Workspaces: row.Workspaces, SessionEndsAt: row.EndsAt}, nil
}

// Renew -.
func (s *PGStore) Renew(ctx context.Context, refreshHash []byte, next Pair) error {
	_, err := s.q.RefreshSession(ctx, db.RefreshSessionParams{
		NewTokenHash: next.TokenHash, NewRefreshHash: next.RefreshHash, EndsAt: next.EndsAt, RefreshEndsAt: next.RefreshEndsAt, RefreshHash: refreshHash,
	})
	if err != nil {
		return notFound(err)
	}

	return nil
}

// Delete -.
func (s *PGStore) Delete(ctx context.Context, tokenHash []byte) error {
	if err := s.q.DeleteSession(ctx, tokenHash); err != nil {
		return fmt.Errorf("session store: delete: %w", err)
	}

	return nil
}

// notFound is problem.ErrNotFound for no row, else the error wrapped.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return problem.ErrNotFound
	}

	return fmt.Errorf("session store: %w", err)
}
