//go:build integration

package projects_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/dbtest"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/projects"
)

// acmeDB has Acme and Beta, and three of Acme's projects made in order: Shop, Blog, Atlas.
func acmeDB(t *testing.T) (*pgxpool.Pool, *projects.PGStore, map[string]string) {
	t.Helper()

	pool := dbtest.New(t)
	_, err := pool.Exec(t.Context(), `INSERT INTO workspaces (id, name) VALUES ('w-acme', 'Acme'), ('w-beta', 'Beta')`)
	require.NoError(t, err)

	s := projects.NewPGStore(pool)
	ids := map[string]string{}

	for _, p := range []*projects.Project{
		{Workspace: "w-acme", Name: "Shop", Description: "The new storefront", Owner: "Olive Owner"},
		{Workspace: "w-acme", Name: "Blog", Description: "Posts", Owner: "Max Member"},
		{Workspace: "w-acme", Name: "Atlas", Description: "50% maps", Owner: "Kim"},
		{Workspace: "w-beta", Name: "Beta site", Description: "", Owner: "Kim"},
	} {
		made, err := s.Create(t.Context(), p)
		require.NoError(t, err)

		ids[p.Name] = made.ID
	}

	shop, err := s.Get(t.Context(), ids["Shop"])
	require.NoError(t, err)

	shop.Tags = []string{"web", "launch"}
	_, err = s.Update(t.Context(), &shop)
	require.NoError(t, err)

	blog, err := s.Get(t.Context(), ids["Blog"])
	require.NoError(t, err)

	blog.Status, blog.Tags = "paused", []string{"web"}
	_, err = s.Update(t.Context(), &blog)
	require.NoError(t, err)

	return pool, s, ids
}

func listed(t *testing.T, s *projects.PGStore, q projects.Query) ([]string, int) {
	t.Helper()

	if q.Workspaces == nil {
		q.Workspaces = []string{"w-acme"}
	}

	if q.Tags == nil {
		q.Tags = []string{}
	}

	if q.Limit == 0 {
		q.Limit = projects.PageSize
	}

	items, total, err := s.List(t.Context(), &q)
	require.NoError(t, err)

	out := []string{}
	for i := range items {
		out = append(out, items[i].Name)
	}

	return out, total
}

func TestPGStore_ListFiltersAsTheQueriesSay(t *testing.T) {
	t.Parallel()

	_, s, _ := acmeDB(t)

	names, total := listed(t, s, projects.Query{})
	assert.Equal(t, []string{"Blog", "Shop", "Atlas"}, names, "the last changed first: Blog, then Shop, were changed after Atlas was made")
	assert.Equal(t, 3, total, "Beta's project is in another workspace")

	names, _ = listed(t, s, projects.Query{Tags: []string{"web", "launch"}})
	assert.Equal(t, []string{"Shop"}, names, "every tag asked")

	names, _ = listed(t, s, projects.Query{Status: "paused"})
	assert.Equal(t, []string{"Blog"}, names)

	names, _ = listed(t, s, projects.Query{Q: "STOREFRONT"})
	assert.Equal(t, []string{"Shop"}, names)

	names, _ = listed(t, s, projects.Query{Q: "50%"})
	assert.Equal(t, []string{"Atlas"}, names, "a % typed is a %, not a pattern")

	names, _ = listed(t, s, projects.Query{Q: "%"})
	assert.Equal(t, []string{"Atlas"}, names)

	names, _ = listed(t, s, projects.Query{Workspaces: []string{"w-acme", "w-beta"}, Q: "kim"})
	assert.Equal(t, []string{"Beta site", "Atlas"}, names)
}

func TestPGStore_ListSortsAndPagesWithTheTotalOfEveryMatch(t *testing.T) {
	t.Parallel()

	_, s, _ := acmeDB(t)

	names, _ := listed(t, s, projects.Query{Sort: "name"})
	assert.Equal(t, []string{"Atlas", "Blog", "Shop"}, names)

	names, _ = listed(t, s, projects.Query{Sort: "name", Descending: true})
	assert.Equal(t, []string{"Shop", "Blog", "Atlas"}, names)

	names, _ = listed(t, s, projects.Query{Sort: "owner", Descending: true})
	assert.Equal(t, []string{"Shop", "Blog", "Atlas"}, names, "Olive, Max, Kim")

	names, total := listed(t, s, projects.Query{Sort: "name", Limit: 2, Offset: 2})
	assert.Equal(t, []string{"Shop"}, names)
	assert.Equal(t, 3, total)

	names, total = listed(t, s, projects.Query{Sort: "name", Limit: 2, Offset: 9})
	assert.Empty(t, names)
	assert.Equal(t, 3, total, "a page past the end still says how many match")
}

func TestPGStore_ANameIsTakenInItsWorkspaceInAnyCase(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	taken, err := s.NameTaken(t.Context(), "w-acme", "SHOP", "")
	require.NoError(t, err)
	assert.True(t, taken)

	taken, err = s.NameTaken(t.Context(), "w-acme", "shop", ids["Shop"])
	require.NoError(t, err)
	assert.False(t, taken, "its own name is no clash")

	taken, err = s.NameTaken(t.Context(), "w-beta", "Shop", "")
	require.NoError(t, err)
	assert.False(t, taken, "another workspace may have one")

	_, err = s.Create(t.Context(), &projects.Project{Workspace: "w-acme", Name: "shop", Owner: "x"})
	require.Error(t, err, "the database refuses it too, whatever the service checked")
}

func TestPGStore_AProjectMadeHasItsDefaults(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	p, err := s.Get(t.Context(), ids["Atlas"])
	require.NoError(t, err)
	assert.Equal(t, "active", p.Status)
	assert.Equal(t, []string{}, p.Tags)
	assert.False(t, p.UpdatedAt.IsZero())

	_, err = s.Get(t.Context(), "not-a-uuid")
	require.ErrorIs(t, err, problem.ErrNotFound, "an id that isn't one is not there, not a 500")
}

func TestPGStore_TagsAreEveryTagInUseOnceSorted(t *testing.T) {
	t.Parallel()

	_, s, _ := acmeDB(t)

	tags, err := s.Tags(t.Context(), []string{"w-acme"})
	require.NoError(t, err)
	assert.Equal(t, []string{"launch", "web"}, tags)

	tags, err = s.Tags(t.Context(), []string{"w-beta"})
	require.NoError(t, err)
	assert.Equal(t, []string{}, tags)
}

func TestPGStore_DeleteRemovesTheProject(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	require.NoError(t, s.Delete(t.Context(), ids["Blog"]))

	_, err := s.Get(t.Context(), ids["Blog"])
	require.ErrorIs(t, err, problem.ErrNotFound)
}
