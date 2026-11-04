package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/projects/db"
)

// PGStore keeps projects in Postgres, through the queries sqlc made of queries.sql.
type PGStore struct {
	q *db.Queries
}

// NewPGStore -.
func NewPGStore(conn db.DBTX) *PGStore { return &PGStore{q: db.New(conn)} }

// List -.
func (s *PGStore) List(ctx context.Context, q *Query) ([]Project, int, error) {
	rows, err := s.q.ListProjects(ctx, db.ListProjectsParams{
		Workspaces: q.Workspaces, Q: q.Q, Status: q.Status, Tags: q.Tags, Sort: q.Sort, Descending: q.Descending,
		Off: int32(q.Offset), Lim: int32(q.Limit), //nolint:gosec // a page's bounds, checked by the service
	})
	if err != nil {
		return nil, 0, fmt.Errorf("projects store: list: %w", err)
	}

	total, err := s.q.CountProjects(ctx, db.CountProjectsParams{Workspaces: q.Workspaces, Q: q.Q, Status: q.Status, Tags: q.Tags})
	if err != nil {
		return nil, 0, fmt.Errorf("projects store: count: %w", err)
	}

	out := make([]Project, 0, len(rows))
	for i := range rows {
		out = append(out, of(&rows[i]))
	}

	return out, int(total), nil
}

// Get -.
func (s *PGStore) Get(ctx context.Context, id string) (Project, error) {
	row, err := s.q.Project(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, problem.ErrNotFound
	}

	if err != nil {
		return Project{}, fmt.Errorf("projects store: get: %w", err)
	}

	return of(&row), nil
}

// NameTaken -.
func (s *PGStore) NameTaken(ctx context.Context, workspace, name, otherThan string) (bool, error) {
	taken, err := s.q.NameTaken(ctx, db.NameTakenParams{WorkspaceID: workspace, Name: name, OtherThan: otherThan})
	if err != nil {
		return false, fmt.Errorf("projects store: name: %w", err)
	}

	return taken, nil
}

// Create -.
func (s *PGStore) Create(ctx context.Context, p *Project) (Project, error) {
	row, err := s.q.CreateProject(ctx, db.CreateProjectParams{WorkspaceID: p.Workspace, Name: p.Name, Description: p.Description, Owner: p.Owner})
	if err != nil {
		return Project{}, fmt.Errorf("projects store: create: %w", err)
	}

	return of(&row), nil
}

// Update -.
func (s *PGStore) Update(ctx context.Context, p *Project) (Project, error) {
	row, err := s.q.UpdateProject(ctx, db.UpdateProjectParams{ID: p.ID, Name: p.Name, Description: p.Description, Status: p.Status, Tags: p.Tags})
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, problem.ErrNotFound
	}

	if err != nil {
		return Project{}, fmt.Errorf("projects store: update: %w", err)
	}

	return of(&row), nil
}

// Delete -.
func (s *PGStore) Delete(ctx context.Context, id string) error {
	if err := s.q.DeleteProject(ctx, id); err != nil {
		return fmt.Errorf("projects store: delete: %w", err)
	}

	return nil
}

// Tags -.
func (s *PGStore) Tags(ctx context.Context, workspaces []string) ([]string, error) {
	tags, err := s.q.Tags(ctx, workspaces)
	if err != nil {
		return nil, fmt.Errorf("projects store: tags: %w", err)
	}

	return tags, nil
}

func of(r *db.Project) Project {
	return Project{ID: r.ID, Workspace: r.WorkspaceID, Name: r.Name, Description: r.Description, Status: r.Status, Owner: r.Owner, Tags: r.Tags, UpdatedAt: r.UpdatedAt}
}
