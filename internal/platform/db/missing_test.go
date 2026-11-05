package db_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"github.com/tinhthanh/go-service-template/internal/platform/db"
)

func TestDB_AWriteNamingARowDeletedMeanwhileIsMissingAndNothingElseIs(t *testing.T) {
	t.Parallel()

	gone := fmt.Errorf("comments store: create: %w", &pgconn.PgError{Code: "23503", ConstraintName: "comments_task_id_fkey"})

	assert.True(t, db.Missing(gone), "wrapped, as a store returns it")
	assert.True(t, db.Missing(gone, "comments_task_id_fkey"))
	assert.False(t, db.Missing(gone, "comments_author_id_fkey"), "another foreign key")
	assert.False(t, db.Missing(&pgconn.PgError{Code: "23505"}), "a unique violation is not a missing row")
	assert.False(t, db.Missing(errors.New("no route to host")))
	assert.False(t, db.Missing(nil))
}
