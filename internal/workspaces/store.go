package workspaces

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tinhthanh/go-service-template/internal/workspaces/db"
)

// PGStore keeps workspaces in Postgres, through the queries sqlc made of queries.sql.
type PGStore struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewPGStore -.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool, q: db.New(pool)} }

// Of -.
func (s *PGStore) Of(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := s.q.WorkspacesOf(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("workspaces store: of: %w", err)
	}

	out := make([]Workspace, 0, len(rows))
	for _, r := range rows {
		out = append(out, Workspace{ID: r.ID, Name: r.Name})
	}

	return out, nil
}

// Taken -.
func (s *PGStore) Taken(ctx context.Context, id string) (bool, error) {
	taken, err := s.q.WorkspaceIDTaken(ctx, id)
	if err != nil {
		return false, fmt.Errorf("workspaces store: taken: %w", err)
	}

	return taken, nil
}

// Create -.
func (s *PGStore) Create(ctx context.Context, w Workspace, member string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{ID: w.ID, Name: w.Name}); err != nil {
			return fmt.Errorf("workspace: %w", err)
		}

		if err := q.Join(ctx, db.JoinParams{UserID: member, WorkspaceID: w.ID}); err != nil {
			return fmt.Errorf("membership: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("workspaces store: create: %w", err)
	}

	return nil
}
