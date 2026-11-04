package tasks_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pages "github.com/tinhthanh/go-service-template/internal/platform/page"
	"github.com/tinhthanh/go-service-template/internal/server/servertest"
	"github.com/tinhthanh/go-service-template/internal/tasks"
)

type task struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Due     string `json:"due"`
	Project *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	Attachment *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Size int    `json:"size"`
		Type string `json:"type"`
	} `json:"attachment"`
	Workspace string `json:"workspace"`
	UpdatedAt string `json:"updatedAt"`
}

type page struct {
	Items   []task `json:"items"`
	Total   int    `json:"total"`
	Next    string `json:"next"`
	Summary struct {
		Late int `json:"late"`
	} `json:"summary"`
}

func list(t *testing.T, s *acmeServer, query string, who map[string]string) page {
	t.Helper()

	a := s.Do(http.MethodGet, "/tasks"+query, "", who)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	var p page
	require.NoError(t, json.Unmarshal(a.Body, &p))

	return p
}

func titles(p page) []string {
	out := []string{}
	for _, it := range p.Items {
		out = append(out, it.Title)
	}

	return out
}

func decode(t *testing.T, a []byte) task {
	t.Helper()

	var out task
	require.NoError(t, json.Unmarshal(a, &out), string(a))

	return out
}

func on(day string) *time.Time {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		panic(err)
	}

	return &d
}

// acmeServer is the server and the ids of Acme's projects.
type acmeServer struct {
	*servertest.Server
	Shop, Blog string
}

// acme is a server (on 2026-11-04) with Acme's projects Shop and Blog (made as the client makes them:
// a feature's tests never import another feature), and four of its tasks an hour apart, the last
// the newest: Checkout (Shop, due in two days), Retries (late), Icons (Blog, done, past its day) and
// Notes (no day).
func acme(t *testing.T) *acmeServer {
	t.Helper()

	s := &acmeServer{Server: servertest.New(t)}
	at := s.Clock().Now()
	owner := s.As(servertest.Owner)

	for name, id := range map[string]*string{"Shop": &s.Shop, "Blog": &s.Blog} {
		a := s.Do(http.MethodPost, "/projects", `{"name": "`+name+`", "description": ""}`, owner)
		require.Equal(t, http.StatusOK, a.Status, string(a.Body))

		*id = decode(t, a.Body).ID
	}

	for i, tk := range []*tasks.Task{
		{ID: "t-checkout", Title: "Fix the checkout's tax line", Status: "doing", Due: on("2026-11-06"), Project: s.Shop},
		{ID: "t-retries", Title: "Retry failed payments", Status: "todo", Due: on("2026-11-03")},
		{ID: "t-icons", Title: "New app icons", Status: "done", Due: on("2026-11-01"), Project: s.Blog},
		{ID: "t-notes", Title: "Write release notes", Status: "todo"},
	} {
		tk.Workspace = servertest.Acme
		tk.UpdatedAt = at.Add(time.Duration(i) * time.Hour)
		s.Tasks.Add(tk)
	}

	return s
}

func TestTasks_AreTheCallersWorkspacesNewestFirstWithHowManyAreLate(t *testing.T) {
	t.Parallel()

	s := acme(t)
	p := list(t, s, "", s.As(servertest.Viewer))

	assert.Equal(t, []string{"Write release notes", "New app icons", "Retry failed payments", "Fix the checkout's tax line"}, titles(p),
		"Open the safe is Initrode's, which the viewer isn't in")
	assert.Equal(t, 4, p.Total)
	assert.Equal(t, 1, p.Summary.Late, "Retries is past its day; Icons is too, but done")
	assert.Empty(t, p.Next, "the last page says no next")
}

func TestTasks_LateIsCountedOverEveryMatchOnTheServersDay(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)

	assert.Equal(t, 1, list(t, s, "?sort=title:asc&limit=1", me).Summary.Late, "Retries isn't on the page, and still counts")
	assert.Equal(t, 0, list(t, s, "?status=doing", me).Summary.Late, "only the matches")

	s.Clock().Advance(3 * 24 * time.Hour)
	assert.Equal(t, 2, list(t, s, "", s.As(servertest.Member)).Summary.Late, "on the 7th, Checkout's day (the 6th) has gone by too")
}

func TestTasks_FilterSortAndPageAsTheClientAsks(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)

	assert.Equal(t, []string{"Fix the checkout's tax line"}, titles(list(t, s, "?q=CHECKOUT", me)), "words in the title, any case")
	assert.Equal(t, []string{"New app icons"}, titles(list(t, s, "?q=blog", me)), "or its project's name")
	assert.Empty(t, titles(list(t, s, "?q=%25", me)), "what was typed is never a pattern")
	assert.Equal(t, []string{"Write release notes", "Retry failed payments"}, titles(list(t, s, "?status=todo", me)))
	assert.Equal(t, []string{"Write release notes", "Retry failed payments", "Fix the checkout's tax line"}, titles(list(t, s, "?hideDone=true", me)))
	assert.Equal(t, []string{"Fix the checkout's tax line"}, titles(list(t, s, "?project="+s.Shop, me)))
	assert.Equal(t, []string{"Fix the checkout's tax line", "New app icons", "Retry failed payments", "Write release notes"}, titles(list(t, s, "?sort=title:asc", me)))
	assert.Equal(t, []string{"Write release notes", "Retry failed payments", "New app icons", "Fix the checkout's tax line"}, titles(list(t, s, "?sort=title:desc", me)))
	assert.Equal(t, []string{"New app icons", "Retry failed payments", "Fix the checkout's tax line", "Write release notes"}, titles(list(t, s, "?sort=due:asc", me)),
		"no day sorts last")
	assert.Equal(t, []string{"Write release notes", "Fix the checkout's tax line", "Retry failed payments", "New app icons"}, titles(list(t, s, "?sort=due:desc", me)),
		"and first, the other way round, as the client's mock reverses it")
	assert.Equal(t, []string{"Fix the checkout's tax line", "Retry failed payments", "New app icons", "Write release notes"}, titles(list(t, s, "?sort=updated:asc", me)))

	first := list(t, s, "?sort=title:asc&limit=3", me)
	assert.Equal(t, []string{"Fix the checkout's tax line", "New app icons", "Retry failed payments"}, titles(first))
	assert.Equal(t, 4, first.Total, "the total is every match, not the page")
	require.Equal(t, "3", first.Next)

	assert.Equal(t, []string{"Write release notes"}, titles(list(t, s, "?sort=title:asc&limit=3&cursor="+first.Next, me)))
}

func TestTasks_ACursorThatIsntOneStartsAtTheTop(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)

	for _, cursor := range []string{"abc", "-1", "99999999999"} {
		assert.Equal(t, []string{"Fix the checkout's tax line"}, titles(list(t, s, "?sort=title:asc&limit=1&cursor="+cursor, me)), cursor)
	}
}

func TestTasks_APageOfMoreThanTheMostIsRefused(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodGet, fmt.Sprintf("/tasks?limit=%d", pages.Max+1), "", s.As(servertest.Member))

	assert.Equal(t, http.StatusUnprocessableEntity, a.Status)
	assert.Contains(t, string(a.Body), `"field":"limit"`)
}

func TestTasks_AnotherTeamsWorkspaceAndTaskAreNotThere(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Owner)

	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/tasks?workspace="+servertest.Initrode, "", me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/tasks/"+servertest.Safe, "", me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodPatch, "/tasks/"+servertest.Safe, `{"title": "Mine"}`, me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodDelete, "/tasks/"+servertest.Safe, "", me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodPost, "/tasks", `{"title": "X", "workspace": "`+servertest.Initrode+`"}`, me).Status)
	assert.Empty(t, titles(list(t, s, "?project="+servertest.Vault, me)), "another's project has none of the caller's tasks")
}

func TestTask_IsOneOfTheCallersWithItsProjectsName(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodGet, "/tasks/t-checkout", "", s.As(servertest.Viewer))

	require.Equal(t, http.StatusOK, a.Status)
	assert.JSONEq(t, `{"id": "t-checkout", "workspace": "w-acme", "title": "Fix the checkout's tax line", "status": "doing",
		"due": "2026-11-06", "project": {"id": "`+s.Shop+`", "name": "Shop"}, "updatedAt": "2026-11-04T09:00:00Z"}`, string(a.Body))

	notes := s.Do(http.MethodGet, "/tasks/t-notes", "", s.As(servertest.Viewer))
	assert.JSONEq(t, `{"id": "t-notes", "workspace": "w-acme", "title": "Write release notes", "status": "todo",
		"updatedAt": "2026-11-04T12:00:00Z"}`, string(notes.Body), "no day and no project are left out")

	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/tasks/t-none", "", s.As(servertest.Viewer)).Status)
}

func TestTask_ShowsItsProjectAsItIsNow(t *testing.T) {
	t.Parallel()

	s := acme(t)
	owner := s.As(servertest.Owner)

	require.Equal(t, http.StatusOK, s.Do(http.MethodPatch, "/projects/"+s.Shop, `{"name": "Storefront"}`, owner).Status)
	assert.Equal(t, "Storefront", decode(t, s.Do(http.MethodGet, "/tasks/t-checkout", "", owner).Body).Project.Name, "renamed")

	require.Equal(t, http.StatusNoContent, s.Do(http.MethodDelete, "/projects/"+s.Shop, "", owner).Status)
	assert.Nil(t, decode(t, s.Do(http.MethodGet, "/tasks/t-checkout", "", owner).Body).Project, "deleted: the task stays, with none")
}

func TestCreateTask_IsToDoInTheMakersFirstWorkspace(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodPost, "/tasks", `{"title": " Plan the launch "}`, s.As(servertest.Member))

	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	made := decode(t, a.Body)
	assert.Equal(t, servertest.Acme, made.Workspace)
	assert.Equal(t, "Plan the launch", made.Title, "trimmed")
	assert.Equal(t, "todo", made.Status)
	assert.Empty(t, made.Due)
	assert.Nil(t, made.Project)

	got := s.Do(http.MethodGet, "/tasks/"+made.ID, "", s.As(servertest.Member))
	assert.Equal(t, http.StatusOK, got.Status, "it is kept")
}

func TestCreateTask_TakesAStatusADayAndAProjectOfItsWorkspace(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodPost, "/tasks", `{"title": "Ship it", "status": "doing", "due": "2026-12-01", "project": "`+s.Blog+`", "attachment": null,
		"workspace": "w-acme"}`, s.As(servertest.Member))

	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	made := decode(t, a.Body)
	assert.Equal(t, "doing", made.Status)
	assert.Equal(t, "2026-12-01", made.Due)
	require.NotNil(t, made.Project)
	assert.Equal(t, "Blog", made.Project.Name)
}

func TestCreateTask_WhatTheRulesRefuseIsInvalidOnItsField(t *testing.T) {
	t.Parallel()

	s := acme(t)

	const (
		rule     = http.StatusUnprocessableEntity // the service's rules, as the mock answers
		contract = http.StatusBadRequest          // the contract's shape, refused before the handler
	)

	for _, c := range []struct {
		body   string
		status int
		want   string
	}{
		{`{"title": "  "}`, rule, `{"error": "A task needs a title.", "code": "invalid", "field": "title"}`},
		{`{"title": "X", "project": "p-vault"}`, rule, `{"error": "Pick a project in this workspace.", "code": "invalid", "field": "project"}`},
		{`{"title": "X", "project": "p-none"}`, rule, `{"error": "Pick a project in this workspace.", "code": "invalid", "field": "project"}`},
		{`{"title": "X", "attachment": "f-1"}`, rule, `{"error": "That file didn't arrive; add it again.", "code": "invalid", "field": "attachment"}`},
		{`{"title": "X", "status": "blocked"}`, contract, `{"error": "This value isn't accepted here.", "code": "invalid", "field": "status"}`},
		{`{"title": "X", "due": "next tuesday"}`, contract, `{"error": "This value isn't accepted here.", "code": "invalid", "field": "due"}`},
		{`{"title": "X", "owner": "Olive Owner"}`, contract, `{"error": "This value isn't accepted here.", "code": "invalid", "field": "owner"}`},
		{`{"status": "todo"}`, contract, `{"error": "This is needed.", "code": "invalid", "field": "title"}`},
	} {
		a := s.Do(http.MethodPost, "/tasks", c.body, s.As(servertest.Member))

		assert.Equal(t, c.status, a.Status, c.body)
		assert.JSONEq(t, c.want, string(a.Body), c.body)
	}

	nobody := s.Do(http.MethodPost, "/tasks", `{"title": "X", "workspace": "w-nobody"}`, s.As(servertest.Member))
	assert.Equal(t, http.StatusNotFound, nobody.Status, "a workspace the maker isn't in")

	// A day the contract's format takes but the calendar doesn't is refused before the handler, with
	// no field: the generated body fails to decode it after the contract is checked.
	impossible := s.Do(http.MethodPost, "/tasks", `{"title": "X", "due": "2026-11-31"}`, s.As(servertest.Member))
	assert.Equal(t, http.StatusBadRequest, impossible.Status)
	assert.Contains(t, string(impossible.Body), `"code":"invalid"`)

	assert.Equal(t, 4, list(t, s, "", s.As(servertest.Member)).Total, "nothing was made")
}

func TestUpdateTask_ChangesWhatItSendsAndKeepsTheRest(t *testing.T) {
	t.Parallel()

	s := acme(t)
	s.Clock().Advance(time.Hour)
	me := s.As(servertest.Member)

	a := s.Do(http.MethodPatch, "/tasks/t-checkout", `{"status": "done", "title": " Fix the tax line "}`, me)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))
	assert.JSONEq(t, `{"id": "t-checkout", "workspace": "w-acme", "title": "Fix the tax line", "status": "done",
		"due": "2026-11-06", "project": {"id": "`+s.Shop+`", "name": "Shop"}, "updatedAt": "2026-11-04T10:00:00Z"}`, string(a.Body))

	moved := s.Do(http.MethodPatch, "/tasks/t-checkout", `{"due": "2026-11-20", "project": "`+s.Blog+`"}`, me)
	require.Equal(t, http.StatusOK, moved.Status, string(moved.Body))
	assert.Equal(t, "2026-11-20", decode(t, moved.Body).Due)
	assert.Equal(t, "Blog", decode(t, moved.Body).Project.Name)
}

func TestUpdateTask_LeavesTheTaskInItsWorkspace(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodPatch, "/tasks/t-notes", `{"workspace": "`+servertest.Initrode+`", "title": "Notes"}`, s.As(servertest.Member))

	require.Equal(t, http.StatusOK, a.Status, string(a.Body))
	assert.Equal(t, servertest.Acme, decode(t, a.Body).Workspace, "a task isn't moved, as the client's mock ignores it")
	assert.Equal(t, "Notes", decode(t, a.Body).Title)
}

func TestUpdateTask_NullTakesADayAProjectOrAFileOff(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)

	a := s.Do(http.MethodPatch, "/tasks/t-checkout", `{"due": null, "project": null, "attachment": null}`, me)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	off := decode(t, a.Body)
	assert.Empty(t, off.Due)
	assert.Nil(t, off.Project)
	assert.Equal(t, "Fix the checkout's tax line", off.Title, "left out is kept")

	got := decode(t, s.Do(http.MethodGet, "/tasks/t-checkout", "", me).Body)
	assert.Empty(t, got.Due, "and it is saved so")
	assert.Nil(t, got.Project)
}

func TestUpdateTask_WhatTheRulesRefuseIsInvalidOnItsField(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)

	for _, c := range []struct {
		body   string
		status int
		want   string
	}{
		{`{"title": ""}`, http.StatusUnprocessableEntity, `{"error": "A task needs a title.", "code": "invalid", "field": "title"}`},
		{`{"project": "p-vault"}`, http.StatusUnprocessableEntity, `{"error": "Pick a project in this workspace.", "code": "invalid", "field": "project"}`},
		{`{"attachment": "f-1"}`, http.StatusUnprocessableEntity, `{"error": "That file didn't arrive; add it again.", "code": "invalid", "field": "attachment"}`},
		{`{"due": "Feb 30"}`, http.StatusBadRequest, `{"error": "This value isn't accepted here.", "code": "invalid", "field": "due"}`},
		{`{"status": "cancelled"}`, http.StatusBadRequest, `{"error": "This value isn't accepted here.", "code": "invalid", "field": "status"}`},
	} {
		a := s.Do(http.MethodPatch, "/tasks/t-checkout", c.body, me)

		assert.Equal(t, c.status, a.Status, c.body)
		assert.JSONEq(t, c.want, string(a.Body), c.body)
	}

	got := decode(t, s.Do(http.MethodGet, "/tasks/t-checkout", "", me).Body)
	assert.Equal(t, "Fix the checkout's tax line", got.Title, "nothing was changed")
	assert.Equal(t, "Shop", got.Project.Name)
}

func TestTasks_EachRoleMayDoWhatTheClientsTableSays(t *testing.T) {
	t.Parallel()

	s := acme(t)
	viewer, member, owner := s.As(servertest.Viewer), s.As(servertest.Member), s.As(servertest.Owner)

	assert.Equal(t, http.StatusForbidden, s.Do(http.MethodPost, "/tasks", `{"title": "X"}`, viewer).Status)
	assert.Equal(t, http.StatusForbidden, s.Do(http.MethodPatch, "/tasks/t-notes", `{"title": "X"}`, viewer).Status)
	assert.Equal(t, http.StatusForbidden, s.Do(http.MethodDelete, "/tasks/t-notes", "", member).Status, "only an owner deletes")

	assert.Equal(t, http.StatusOK, s.Do(http.MethodPatch, "/tasks/t-notes", `{"status": "doing"}`, member).Status)
	assert.Equal(t, http.StatusNoContent, s.Do(http.MethodDelete, "/tasks/t-notes", "", owner).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/tasks/t-notes", "", owner).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodDelete, "/tasks/t-notes", "", owner).Status, "a task deleted isn't there")
}

// sent uploads a file as the person, as the client does before it names it on a task.
func sent(t *testing.T, s *acmeServer, who map[string]string, name string) string {
	t.Helper()

	header := map[string]string{"X-File-Name": name, "Content-Type": "application/pdf"}
	maps.Copy(header, who)

	a := s.Do(http.MethodPost, "/files", "%PDF-1.7 a plan", header)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	return decode(t, a.Body).ID
}

func TestTask_CarriesTheFileItsMakerSent(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)
	file := sent(t, s, me, "plan.pdf")

	a := s.Do(http.MethodPost, "/tasks", `{"title": "Read the plan", "attachment": "`+file+`"}`, me)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	made := decode(t, a.Body)
	require.NotNil(t, made.Attachment)
	assert.Equal(t, file, made.Attachment.ID)
	assert.Equal(t, "plan.pdf", made.Attachment.Name)
	assert.Equal(t, "application/pdf", made.Attachment.Type)
	assert.Equal(t, len("%PDF-1.7 a plan"), made.Attachment.Size)

	again := decode(t, s.Do(http.MethodGet, "/tasks/"+made.ID, "", s.As(servertest.Viewer)).Body)
	require.NotNil(t, again.Attachment, "read back by anyone who may read the task")
	assert.Equal(t, file, again.Attachment.ID)
}

func TestTask_AFileSomeoneElseSentIsRefusedOnTheField(t *testing.T) {
	t.Parallel()

	s := acme(t)
	theirs := sent(t, s, s.As(servertest.Owner), "owner.pdf")

	a := s.Do(http.MethodPost, "/tasks", `{"title": "Take theirs", "attachment": "`+theirs+`"}`, s.As(servertest.Member))

	assert.Equal(t, http.StatusUnprocessableEntity, a.Status)
	assert.JSONEq(t, `{"error": "That file didn't arrive; add it again.", "code": "invalid", "field": "attachment"}`, string(a.Body))
}

func TestUpdateTask_NullTakesTheFileOffAndTheSameFileStays(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)
	file := sent(t, s, me, "plan.pdf")

	made := decode(t, s.Do(http.MethodPost, "/tasks", `{"title": "Read the plan", "attachment": "`+file+`"}`, me).Body)

	// The owner changes the title and sends the file the task already has: it stays, though it isn't theirs.
	kept := s.Do(http.MethodPatch, "/tasks/"+made.ID, `{"title": "Read the whole plan", "attachment": "`+file+`"}`, s.As(servertest.Owner))
	require.Equal(t, http.StatusOK, kept.Status, string(kept.Body))
	assert.NotNil(t, decode(t, kept.Body).Attachment)

	off := s.Do(http.MethodPatch, "/tasks/"+made.ID, `{"attachment": null}`, me)
	require.Equal(t, http.StatusOK, off.Status, string(off.Body))
	assert.Nil(t, decode(t, off.Body).Attachment)
}
