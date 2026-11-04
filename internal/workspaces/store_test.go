//go:build integration

package workspaces_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/dbtest"
	"github.com/tinhthanh/go-service-template/internal/workspaces"
)

func TestPGStore_CreateMakesTheWorkspaceWithItsMakerInIt(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)

	var maxID string
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (name, email, password_hash, role) VALUES ('Max', 'max@acme.test', 'x', 'member') RETURNING id`).Scan(&maxID))

	s := workspaces.NewPGStore(pool)
	require.NoError(t, s.Create(t.Context(), workspaces.Workspace{ID: "w-zeta", Name: "zeta"}, maxID))
	require.NoError(t, s.Create(t.Context(), workspaces.Workspace{ID: "w-acme", Name: "Acme"}, maxID))

	mine, err := s.Of(t.Context(), maxID)
	require.NoError(t, err)
	assert.Equal(t, []workspaces.Workspace{{ID: "w-acme", Name: "Acme"}, {ID: "w-zeta", Name: "zeta"}}, mine, "by name, in any case")

	taken, err := s.Taken(t.Context(), "w-acme")
	require.NoError(t, err)
	assert.True(t, taken)
}

func TestPGStore_AWorkspaceIsNotMadeWhenItsMakerCantJoin(t *testing.T) {
	t.Parallel()

	pool := dbtest.New(t)
	s := workspaces.NewPGStore(pool)

	// Nobody has this id, so the membership fails: the workspace must not be left behind.
	require.Error(t, s.Create(t.Context(), workspaces.Workspace{ID: "w-orphan", Name: "Orphan"}, "00000000-0000-0000-0000-000000000000"))

	taken, err := s.Taken(t.Context(), "w-orphan")
	require.NoError(t, err)
	assert.False(t, taken, "at once or not at all")
}
