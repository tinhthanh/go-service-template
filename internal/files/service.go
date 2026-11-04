// Package files is a file a person sends, kept as it came: its name (sent
// URI-encoded in X-File-Name), its type, its size and its bytes, up to
// MaxSize, as the client's lib/upload.ts allows. A file is its sender's: a
// task may name only its maker's own (internal/tasks reads this table to
// check, and never writes it). Nothing reads a file's bytes back yet: the
// contract has no download.
package files

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// MaxSize is the most a file may hold: 5 MB, as the client's lib/upload.ts MAX_FILE.
const MaxSize = 5 << 20

// Attachment is a file kept, as a task shows it.
type Attachment struct {
	ID, Name, Type string
	Size           int
}

// Store is what the service keeps files in (store.go, on Postgres).
type Store interface {
	Create(ctx context.Context, owner string, a *Attachment, data []byte) (Attachment, error)
}

// Service -.
type Service struct {
	store Store
}

// New -.
func New(store Store) *Service { return &Service{store: store} }

// Upload keeps the file the caller sends: its name as the client encoded it,
// its type ("application/octet-stream" for none), refused past MaxSize.
func (s *Service) Upload(ctx context.Context, encodedName, contentType string, body io.Reader) (Attachment, error) {
	p, err := auth.Required(ctx)
	if err != nil {
		return Attachment{}, err
	}

	name, err := url.PathUnescape(encodedName)
	if err != nil {
		name = encodedName
	}

	if name = strings.TrimSpace(name); name == "" {
		return Attachment{}, problem.Invalid("attachment", "A file needs a name.")
	}

	data, err := io.ReadAll(io.LimitReader(body, MaxSize+1))
	if err != nil {
		return Attachment{}, fmt.Errorf("files: read: %w", err) // past the server's own limit, a MaxBytesError: a 413
	}

	if len(data) > MaxSize {
		return Attachment{}, problem.Invalid("attachment", "Files up to 5 MB, please.")
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	made, err := s.store.Create(ctx, p.ID, &Attachment{Name: name, Type: contentType, Size: len(data)}, data)
	if err != nil {
		return Attachment{}, fmt.Errorf("files: keep: %w", err)
	}

	return made, nil
}
