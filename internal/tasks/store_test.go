//go:build integration

package tasks_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/dbtest"
	pages "github.com/tinhthanh/go-service-template/internal/platform/page"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/tasks"
)

// acmeDB has Acme with its project Shop and Beta with Beta site, and Acme's tasks changed an hour
// apart, the last the newest: Checkout (Shop, due the 6th), Retries (due the 3rd), Icons (done,
// due the 1st) and Banner (no day); and one of Beta's.
func acmeDB(t *testing.T) (*pgxpool.Pool, *tasks.PGStore, map[string]string) {
	t.Helper()

	pool := dbtest.New(t)
	_, err := pool.Exec(t.Context(), `INSERT INTO workspaces (id, name) VALUES ('w-acme', 'Acme'), ('w-beta', 'Beta')`)
	require.NoError(t, err)

	ids := map[string]string{}

	for ws, name := range map[string]string{"w-acme": "Shop", "w-beta": "Beta site"} {
		var id string
		require.NoError(t, pool.QueryRow(t.Context(),
			`INSERT INTO projects (workspace_id, name, owner) VALUES ($1, $2, 'Kim') RETURNING id`, ws, name).Scan(&id))

		ids[name] = id
	}

	s := tasks.NewPGStore(pool)
	at := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)

	for i, tk := range []*tasks.Task{
		{Workspace: "w-acme", Title: "Fix the checkout", Status: "doing", Due: on("2026-11-06"), Project: ids["Shop"]},
		{Workspace: "w-acme", Title: "Retry payments", Status: "todo", Due: on("2026-11-03")},
		{Workspace: "w-acme", Title: "New icons", Status: "done", Due: on("2026-11-01")},
		{Workspace: "w-acme", Title: "50% off banner", Status: "todo"},
		{Workspace: "w-beta", Title: "Beta launch", Status: "todo", Project: ids["Beta site"]},
	} {
		made, err := s.Create(t.Context(), tk)
		require.NoError(t, err)

		_, err = pool.Exec(t.Context(), `UPDATE tasks SET updated_at = $1 WHERE id = $2`, at.Add(time.Duration(i)*time.Hour), made.ID)
		require.NoError(t, err)

		ids[tk.Title] = made.ID
	}

	return pool, s, ids
}

func listedTasks(t *testing.T, s *tasks.PGStore, q tasks.Query) ([]string, tasks.Counts) {
	t.Helper()

	if q.Workspaces == nil {
		q.Workspaces = []string{"w-acme"}
	}

	if q.Limit == 0 {
		q.Limit = pages.Size
	}

	q.Today = *on("2026-11-04")

	items, counts, err := s.List(t.Context(), &q)
	require.NoError(t, err)

	out := []string{}
	for i := range items {
		out = append(out, items[i].Title)
	}

	return out, counts
}

func TestPGStore_ListFiltersAndCountsTheLateAsTheQueriesSay(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	titles, counts := listedTasks(t, s, tasks.Query{})
	assert.Equal(t, []string{"50% off banner", "New icons", "Retry payments", "Fix the checkout"}, titles, "the last changed first")
	assert.Equal(t, tasks.Counts{Total: 4, Late: 1}, counts, "Beta's is another workspace's; Icons is past its day, but done")

	titles, _ = listedTasks(t, s, tasks.Query{Q: "SHOP"})
	assert.Equal(t, []string{"Fix the checkout"}, titles, "words in its project's name, any case")

	titles, _ = listedTasks(t, s, tasks.Query{Q: "%"})
	assert.Equal(t, []string{"50% off banner"}, titles, "a % typed is a %, not a pattern")

	titles, counts = listedTasks(t, s, tasks.Query{Status: "todo"})
	assert.Equal(t, []string{"50% off banner", "Retry payments"}, titles)
	assert.Equal(t, tasks.Counts{Total: 2, Late: 1}, counts)

	titles, _ = listedTasks(t, s, tasks.Query{HideDone: true})
	assert.Equal(t, []string{"50% off banner", "Retry payments", "Fix the checkout"}, titles)

	titles, _ = listedTasks(t, s, tasks.Query{Project: ids["Shop"]})
	assert.Equal(t, []string{"Fix the checkout"}, titles)

	titles, _ = listedTasks(t, s, tasks.Query{Project: "not-a-uuid"})
	assert.Empty(t, titles, "a project that isn't one has no tasks, not a 500")

	titles, _ = listedTasks(t, s, tasks.Query{Workspaces: []string{"w-acme", "w-beta"}, Q: "launch"})
	assert.Equal(t, []string{"Beta launch"}, titles)
}

func TestPGStore_ListSortsAndPagesWithTheCountsOfEveryMatch(t *testing.T) {
	t.Parallel()

	_, s, _ := acmeDB(t)

	titles, _ := listedTasks(t, s, tasks.Query{Sort: "title"})
	assert.Equal(t, []string{"50% off banner", "Fix the checkout", "New icons", "Retry payments"}, titles)

	titles, _ = listedTasks(t, s, tasks.Query{Sort: "title", Descending: true})
	assert.Equal(t, []string{"Retry payments", "New icons", "Fix the checkout", "50% off banner"}, titles)

	titles, _ = listedTasks(t, s, tasks.Query{Sort: "due"})
	assert.Equal(t, []string{"New icons", "Retry payments", "Fix the checkout", "50% off banner"}, titles, "no day last")

	titles, _ = listedTasks(t, s, tasks.Query{Sort: "due", Descending: true})
	assert.Equal(t, []string{"50% off banner", "Fix the checkout", "Retry payments", "New icons"}, titles, "and first, the other way")

	titles, _ = listedTasks(t, s, tasks.Query{Sort: "updated"})
	assert.Equal(t, []string{"Fix the checkout", "Retry payments", "New icons", "50% off banner"}, titles)

	titles, counts := listedTasks(t, s, tasks.Query{Sort: "title", Limit: 3, Offset: 3})
	assert.Equal(t, []string{"Retry payments"}, titles)
	assert.Equal(t, tasks.Counts{Total: 4, Late: 1}, counts, "the late of every match, not the page's")

	titles, counts = listedTasks(t, s, tasks.Query{Sort: "title", Limit: 3, Offset: 9})
	assert.Empty(t, titles)
	assert.Equal(t, 4, counts.Total, "a page past the end still says how many match")
}

func TestPGStore_ATaskIsReadWithItsProjectsName(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	tk, err := s.Get(t.Context(), ids["Fix the checkout"])
	require.NoError(t, err)
	assert.Equal(t, "w-acme", tk.Workspace)
	assert.Equal(t, "doing", tk.Status)
	assert.Equal(t, on("2026-11-06"), tk.Due)
	assert.Equal(t, ids["Shop"], tk.Project)
	assert.Equal(t, "Shop", tk.ProjectName)

	banner, err := s.Get(t.Context(), ids["50% off banner"])
	require.NoError(t, err)
	assert.Nil(t, banner.Due)
	assert.Empty(t, banner.Project)

	_, err = s.Get(t.Context(), "not-a-uuid")
	require.ErrorIs(t, err, problem.ErrNotFound, "an id that isn't one is not there, not a 500")
}

func TestPGStore_AProjectIsNamedOnlyInItsOwnWorkspace(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	name, err := s.ProjectName(t.Context(), "w-acme", ids["Shop"])
	require.NoError(t, err)
	assert.Equal(t, "Shop", name)

	_, err = s.ProjectName(t.Context(), "w-acme", ids["Beta site"])
	require.ErrorIs(t, err, problem.ErrNotFound, "another workspace's")

	_, err = s.ProjectName(t.Context(), "w-acme", "not-a-uuid")
	require.ErrorIs(t, err, problem.ErrNotFound)
}

func TestPGStore_UpdateSavesTheTaskAsGiven(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	tk, err := s.Get(t.Context(), ids["Fix the checkout"])
	require.NoError(t, err)

	tk.Title, tk.Status, tk.Due, tk.Project = "Fix the tax line", "done", nil, ""
	saved, err := s.Update(t.Context(), &tk)
	require.NoError(t, err)
	assert.Equal(t, "Fix the tax line", saved.Title)
	assert.Equal(t, "done", saved.Status)
	assert.Nil(t, saved.Due, "a day taken off")
	assert.Empty(t, saved.Project, "a project taken off")
	assert.NotEqual(t, tk.UpdatedAt, saved.UpdatedAt, "changed now")

	saved.Due, saved.Project = on("2026-12-24"), ids["Shop"]
	saved, err = s.Update(t.Context(), &saved)
	require.NoError(t, err)
	assert.Equal(t, on("2026-12-24"), saved.Due)
	assert.Equal(t, "Shop", saved.ProjectName)

	_, err = s.Update(t.Context(), &tasks.Task{ID: "not-a-uuid", Title: "X", Status: "todo"})
	require.ErrorIs(t, err, problem.ErrNotFound)
}

func TestPGStore_ATaskOutlivesItsProjectButNotItsWorkspace(t *testing.T) {
	t.Parallel()

	pool, s, ids := acmeDB(t)

	_, err := pool.Exec(t.Context(), `DELETE FROM projects WHERE id = $1`, ids["Shop"])
	require.NoError(t, err)

	tk, err := s.Get(t.Context(), ids["Fix the checkout"])
	require.NoError(t, err)
	assert.Empty(t, tk.Project, "its project deleted, it has none")

	_, err = pool.Exec(t.Context(), `DELETE FROM workspaces WHERE id = 'w-beta'`)
	require.NoError(t, err)

	_, err = s.Get(t.Context(), ids["Beta launch"])
	require.ErrorIs(t, err, problem.ErrNotFound)
}

func TestPGStore_TheDatabaseRefusesWhatTheServiceWould(t *testing.T) {
	t.Parallel()

	_, s, _ := acmeDB(t)

	_, err := s.Create(t.Context(), &tasks.Task{Workspace: "w-acme", Title: "X", Status: "blocked"})
	require.Error(t, err, "a status that isn't one")

	_, err = s.Create(t.Context(), &tasks.Task{Workspace: "w-acme", Title: "", Status: "todo"})
	require.Error(t, err, "no title")
}

func TestPGStore_DeleteRemovesTheTask(t *testing.T) {
	t.Parallel()

	_, s, ids := acmeDB(t)

	require.NoError(t, s.Delete(t.Context(), ids["New icons"]))

	_, err := s.Get(t.Context(), ids["New icons"])
	require.ErrorIs(t, err, problem.ErrNotFound)
}

// A task is found by its primary key's index: the store's own query, as queries.sql writes it,
// compares ids as UUIDs, never as text.
func TestPGStore_ATaskIsFoundByItsKeysIndex(t *testing.T) {
	t.Parallel()

	pool, _, ids := acmeDB(t)

	tx, err := pool.Begin(t.Context())
	require.NoError(t, err)

	defer func() { _ = tx.Rollback(t.Context()) }()

	_, err = tx.Exec(t.Context(), "SET LOCAL enable_seqscan = off")
	require.NoError(t, err)

	src, err := os.ReadFile("queries.sql")
	require.NoError(t, err)

	m := regexp.MustCompile(`(?s)-- name: Task :one\n(.*?);`).FindSubmatch(src)
	require.NotNil(t, m, "queries.sql has the Task query")

	query := regexp.MustCompile(`@id(::\w+)?`).ReplaceAllString(string(m[1]), "$$1")

	// The whole plan: a join's first line is the join, its index scan below.
	rows, err := tx.Query(t.Context(), "EXPLAIN "+query, ids["Fix the checkout"])
	require.NoError(t, err)

	lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)

	plan := strings.Join(lines, "\n")
	assert.Contains(t, plan, "tasks_pkey", query)
}

func TestPGStore_AnIDThatIsntOneIsNoTaskAndNoProject(t *testing.T) {
	t.Parallel()

	_, s, _ := acmeDB(t)

	_, err := s.Get(t.Context(), "not-a-uuid")
	require.ErrorIs(t, err, problem.ErrNotFound)

	_, err = s.ProjectName(t.Context(), "w-acme", "not-a-uuid")
	require.ErrorIs(t, err, problem.ErrNotFound)

	titles, counts := listedTasks(t, s, tasks.Query{Project: "not-a-uuid"})
	assert.Empty(t, titles)
	assert.Zero(t, counts.Total)
}

// file is a file Max sent (another of Kim's, when kim), in the test's database.
func file(t *testing.T, pool *pgxpool.Pool, email string) (string, string) {
	t.Helper()

	var owner, id string
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO users (name, email, password_hash, role) VALUES ($1, $1, 'x', 'member') RETURNING id`, email).Scan(&owner))
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO files (owner_id, name, type, size, data) VALUES ($1, 'plan.pdf', 'application/pdf', 4, 'plan') RETURNING id`, owner).Scan(&id))

	return owner, id
}

func TestPGStore_ATaskIsReadWithItsFileAndNamesOnlyItsMakersOwn(t *testing.T) {
	t.Parallel()

	pool, s, ids := acmeDB(t)
	maxID, plan := file(t, pool, "max@acme.test")
	kim, _ := file(t, pool, "kim@acme.test")

	a, err := s.AttachmentOf(t.Context(), plan, maxID)
	require.NoError(t, err)
	assert.Equal(t, tasks.Attachment{ID: plan, Name: "plan.pdf", Type: "application/pdf", Size: 4}, a)

	_, err = s.AttachmentOf(t.Context(), plan, kim)
	require.ErrorIs(t, err, problem.ErrNotFound, "another's file is not theirs to name")

	_, err = s.AttachmentOf(t.Context(), "not-a-uuid", maxID)
	require.ErrorIs(t, err, problem.ErrNotFound)

	tk, err := s.Get(t.Context(), ids["Retry payments"])
	require.NoError(t, err)

	tk.Attachment = &a
	saved, err := s.Update(t.Context(), &tk)
	require.NoError(t, err)
	require.NotNil(t, saved.Attachment)
	assert.Equal(t, a, *saved.Attachment)

	_, err = pool.Exec(t.Context(), `DELETE FROM files WHERE id = $1`, plan)
	require.NoError(t, err)

	gone, err := s.Get(t.Context(), ids["Retry payments"])
	require.NoError(t, err, "deleting the file leaves the task")
	assert.Nil(t, gone.Attachment)
}
