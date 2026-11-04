package search

import (
	"context"
	"fmt"

	"github.com/tinhthanh/go-service-template/internal/search/db"
)

// PGStore finds records in Postgres, through the queries sqlc made of queries.sql.
type PGStore struct {
	q *db.Queries
}

// NewPGStore -.
func NewPGStore(conn db.DBTX) *PGStore { return &PGStore{q: db.New(conn)} }

// Find -.
func (s *PGStore) Find(ctx context.Context, q string, workspaces []string, per int) ([]Hit, error) {
	limit := int32(per) //nolint:gosec // PerKind, a handful

	projects, err := s.q.SearchProjects(ctx, db.SearchProjectsParams{Q: q, Workspaces: workspaces, Per: limit})
	if err != nil {
		return nil, fmt.Errorf("search store: projects: %w", err)
	}

	tasks, err := s.q.SearchTasks(ctx, db.SearchTasksParams{Q: q, Workspaces: workspaces, Per: limit})
	if err != nil {
		return nil, fmt.Errorf("search store: tasks: %w", err)
	}

	hits := make([]Hit, 0, len(projects)+len(tasks))
	for _, r := range projects {
		hits = append(hits, Hit{Kind: "project", ID: r.ID, Name: r.Name, Detail: r.Detail, Status: r.Status})
	}

	for _, r := range tasks {
		hits = append(hits, Hit{Kind: "task", ID: r.ID, Name: r.Name, Detail: r.Detail, Status: r.Status})
	}

	return hits, nil
}
