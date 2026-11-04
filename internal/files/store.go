package files

import (
	"context"
	"fmt"

	"github.com/tinhthanh/go-service-template/internal/files/db"
)

// PGStore keeps files in Postgres, through the queries sqlc made of queries.sql.
type PGStore struct {
	q *db.Queries
}

// NewPGStore -.
func NewPGStore(conn db.DBTX) *PGStore { return &PGStore{q: db.New(conn)} }

// Create -.
func (s *PGStore) Create(ctx context.Context, owner string, a *Attachment, data []byte) (Attachment, error) {
	row, err := s.q.CreateFile(ctx, db.CreateFileParams{
		OwnerID: owner, Name: a.Name, Type: a.Type, Size: int32(a.Size), Data: data, //nolint:gosec // MaxSize, checked by the service
	})
	if err != nil {
		return Attachment{}, fmt.Errorf("files store: create: %w", err)
	}

	return Attachment{ID: row.ID, Name: row.Name, Type: row.Type, Size: int(row.Size)}, nil
}
