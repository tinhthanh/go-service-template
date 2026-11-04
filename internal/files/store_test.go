//go:build integration

package files_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/files"
	"github.com/tinhthanh/go-service-template/internal/platform/dbtest"
)

func TestPGStore_KeepsTheFilesBytesAsTheyCame(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)

	var maxID string
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (name, email, password_hash, role) VALUES ('Max', 'max@acme.test', 'x', 'member') RETURNING id`).Scan(&maxID))

	data := []byte("%PDF-1.7\x00\xffa plan")
	made, err := files.NewPGStore(pool).Create(t.Context(), maxID, &files.Attachment{Name: "plan.pdf", Type: "application/pdf", Size: len(data)}, data)
	require.NoError(t, err)

	assert.NotEmpty(t, made.ID)
	assert.Equal(t, files.Attachment{ID: made.ID, Name: "plan.pdf", Type: "application/pdf", Size: len(data)}, made)

	var kept []byte
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT data FROM files WHERE id = $1 AND owner_id = $2`, made.ID, maxID).Scan(&kept))
	assert.Equal(t, data, kept, "every byte, a NUL and a 0xff included")

	_, err = files.NewPGStore(pool).Create(t.Context(), maxID, &files.Attachment{Name: "", Type: "x"}, nil)
	require.Error(t, err, "the database refuses a file with no name too")
}
