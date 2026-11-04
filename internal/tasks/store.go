package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/tasks/db"
)

// PGStore keeps tasks in Postgres, through the queries sqlc made of queries.sql.
// An id that isn't a UUID is no task or project: it is answered without a
// query, so the queries compare ids as UUIDs and the keys' indexes serve them.
type PGStore struct {
	q *db.Queries
}

// NewPGStore -.
func NewPGStore(conn db.DBTX) *PGStore { return &PGStore{q: db.New(conn)} }

// List -.
func (s *PGStore) List(ctx context.Context, q *Query) ([]Task, Counts, error) {
	var project *string

	if q.Project != "" {
		if !isUUID(q.Project) {
			return []Task{}, Counts{}, nil // a project that can't be has no tasks
		}

		project = &q.Project
	}

	rows, err := s.q.ListTasks(ctx, db.ListTasksParams{
		Workspaces: q.Workspaces, Q: q.Q, Status: q.Status, HideDone: q.HideDone, Project: project,
		Sort: q.Sort, Descending: q.Descending,
		Off: int32(q.Offset), Lim: int32(q.Limit), //nolint:gosec // a page's bounds, checked by the service
	})
	if err != nil {
		return nil, Counts{}, fmt.Errorf("tasks store: list: %w", err)
	}

	counts, err := s.q.CountTasks(ctx, db.CountTasksParams{
		Today: date(&q.Today), Workspaces: q.Workspaces, Q: q.Q, Status: q.Status, HideDone: q.HideDone, Project: project,
	})
	if err != nil {
		return nil, Counts{}, fmt.Errorf("tasks store: count: %w", err)
	}

	out := make([]Task, 0, len(rows))
	for i := range rows {
		out = append(out, of(db.TaskRow(rows[i])))
	}

	return out, Counts{Total: int(counts.Total), Late: int(counts.Late)}, nil
}

// Get -.
func (s *PGStore) Get(ctx context.Context, id string) (Task, error) {
	if uuid.Validate(id) != nil {
		return Task{}, problem.ErrNotFound
	}

	row, err := s.q.Task(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, problem.ErrNotFound
	}

	if err != nil {
		return Task{}, fmt.Errorf("tasks store: get: %w", err)
	}

	return of(row), nil
}

// ProjectName -.
func (s *PGStore) ProjectName(ctx context.Context, workspace, id string) (string, error) {
	if uuid.Validate(id) != nil {
		return "", problem.ErrNotFound
	}

	name, err := s.q.ProjectName(ctx, db.ProjectNameParams{ID: id, WorkspaceID: workspace})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", problem.ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf("tasks store: a project: %w", err)
	}

	return name, nil
}

// Create -.
func (s *PGStore) Create(ctx context.Context, t *Task) (Task, error) {
	id, err := s.q.CreateTask(ctx, db.CreateTaskParams{
		WorkspaceID: t.Workspace, Title: t.Title, Status: t.Status, Due: date(t.Due), ProjectID: t.Project, AttachmentID: attachmentID(t),
	})
	if err != nil {
		return Task{}, fmt.Errorf("tasks store: create: %w", err)
	}

	return s.Get(ctx, id)
}

// Update -.
func (s *PGStore) Update(ctx context.Context, t *Task) (Task, error) {
	if uuid.Validate(t.ID) != nil {
		return Task{}, problem.ErrNotFound
	}

	id, err := s.q.UpdateTask(ctx, db.UpdateTaskParams{ID: t.ID, Title: t.Title, Status: t.Status, Due: date(t.Due), ProjectID: t.Project, AttachmentID: attachmentID(t)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, problem.ErrNotFound
	}

	if err != nil {
		return Task{}, fmt.Errorf("tasks store: update: %w", err)
	}

	return s.Get(ctx, id)
}

// Delete -.
func (s *PGStore) Delete(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return problem.ErrNotFound
	}

	if err := s.q.DeleteTask(ctx, id); err != nil {
		return fmt.Errorf("tasks store: delete: %w", err)
	}

	return nil
}

func of(r db.TaskRow) Task {
	t := Task{
		ID: r.ID, Workspace: r.WorkspaceID, Title: r.Title, Status: r.Status,
		Project: r.ProjectID, ProjectName: r.ProjectName, UpdatedAt: r.UpdatedAt,
	}

	if r.AttachmentID != "" {
		t.Attachment = &Attachment{ID: r.AttachmentID, Name: r.AttachmentName, Type: r.AttachmentType, Size: int(r.AttachmentSize)}
	}

	if r.Due.Valid {
		d := r.Due.Time.UTC()
		t.Due = &d
	}

	return t
}

// AttachmentOf -.
func (s *PGStore) AttachmentOf(ctx context.Context, id, owner string) (Attachment, error) {
	if !isUUID(id) {
		return Attachment{}, problem.ErrNotFound
	}

	row, err := s.q.AttachmentOf(ctx, db.AttachmentOfParams{ID: id, OwnerID: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, problem.ErrNotFound
	}

	if err != nil {
		return Attachment{}, fmt.Errorf("tasks store: a file: %w", err)
	}

	return Attachment{ID: row.ID, Name: row.Name, Type: row.Type, Size: int(row.Size)}, nil
}

func attachmentID(t *Task) string {
	if t.Attachment == nil {
		return ""
	}

	return t.Attachment.ID
}

func isUUID(s string) bool { return uuid.Validate(s) == nil }

// date is a day for Postgres; nil is NULL.
func date(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}

	return pgtype.Date{Time: *t, Valid: true}
}
