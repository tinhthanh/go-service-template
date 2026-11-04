//go:build integration

package session_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/dbtest"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/session"
)

// person makes Max, a member of w-acme and w-beta, and answers their id.
func person(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()

	var id string
	require.NoError(t, pool.QueryRow(t.Context(), `
WITH u AS (INSERT INTO users (name, email, password_hash, role) VALUES ('Max Member', 'Max@Acme.test', 'hash', 'member') RETURNING id),
     w AS (INSERT INTO workspaces (id, name) VALUES ('w-acme', 'Acme'), ('w-beta', 'Beta'))
INSERT INTO memberships (user_id, workspace_id) SELECT id, ws FROM u, unnest(ARRAY['w-beta', 'w-acme']) AS ws RETURNING user_id`).Scan(&id))

	return id
}

func pair(token, refresh string, ends time.Time) session.Pair {
	return session.Pair{TokenHash: []byte(token), RefreshHash: []byte(refresh), EndsAt: ends, RefreshEndsAt: ends.Add(time.Hour)}
}

func TestPGStore_AnAccountIsFoundByEmailInAnyCase(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)
	id := person(t, pool)
	s := session.NewPGStore(pool)

	acc, err := s.Account(t.Context(), "max@ACME.test")
	require.NoError(t, err)
	assert.Equal(t, id, acc.ID)

	_, err = s.Account(t.Context(), "nobody@acme.test")
	require.ErrorIs(t, err, problem.ErrNotFound)
}

func TestPGStore_ALiveSessionNamesThePersonAndTheirWorkspaces(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)
	id := person(t, pool)
	s := session.NewPGStore(pool)
	ends := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)

	require.NoError(t, s.Create(t.Context(), id, pair("t1", "r1", ends)))

	p, err := s.Person(t.Context(), []byte("t1"))
	require.NoError(t, err)
	assert.Equal(t, "Max Member", p.Name)
	assert.Equal(t, []string{"w-acme", "w-beta"}, p.Workspaces, "sorted")
	assert.True(t, ends.Equal(p.SessionEndsAt))
}

func TestPGStore_AnEndedSessionNamesNobody(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)
	s := session.NewPGStore(pool)

	require.NoError(t, s.Create(t.Context(), person(t, pool), pair("t1", "r1", time.Now().Add(-time.Second))))

	_, err := s.Person(t.Context(), []byte("t1"))
	require.ErrorIs(t, err, problem.ErrNotFound)
}

func TestPGStore_RenewSwapsBothTokensOnce(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)
	s := session.NewPGStore(pool)
	ends := time.Now().Add(time.Hour)

	require.NoError(t, s.Create(t.Context(), person(t, pool), pair("t1", "r1", ends)))
	require.NoError(t, s.Renew(t.Context(), []byte("r1"), pair("t2", "r2", ends)))

	_, err := s.Person(t.Context(), []byte("t1"))
	require.ErrorIs(t, err, problem.ErrNotFound, "the old session token stops")

	_, err = s.Person(t.Context(), []byte("t2"))
	require.NoError(t, err)

	require.ErrorIs(t, s.Renew(t.Context(), []byte("r1"), pair("t3", "r3", ends)), problem.ErrNotFound, "a refresh token works once")
}

func TestPGStore_DeleteEndsTheSession(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)
	s := session.NewPGStore(pool)

	require.NoError(t, s.Create(t.Context(), person(t, pool), pair("t1", "r1", time.Now().Add(time.Hour))))
	require.NoError(t, s.Delete(t.Context(), []byte("t1")))

	_, err := s.Person(t.Context(), []byte("t1"))
	require.ErrorIs(t, err, problem.ErrNotFound)
}
