package projects_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pages "github.com/tinhthanh/go-service-template/internal/platform/page"
	"github.com/tinhthanh/go-service-template/internal/projects"
	"github.com/tinhthanh/go-service-template/internal/server/servertest"
)

type page struct {
	Items []struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Owner  string   `json:"owner"`
		Status string   `json:"status"`
		Tags   []string `json:"tags"`
	} `json:"items"`
	Total int    `json:"total"`
	Next  string `json:"next"`
}

func list(t *testing.T, s *servertest.Server, query string, who map[string]string) page {
	t.Helper()

	a := s.Do(http.MethodGet, "/projects"+query, "", who)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	var p page
	require.NoError(t, json.Unmarshal(a.Body, &p))

	return p
}

func names(p page) []string {
	out := []string{}
	for _, it := range p.Items {
		out = append(out, it.Name)
	}

	return out
}

// acme is a server with Acme's projects: Shop, Blog and Atlas, an hour apart, Atlas the newest.
func acme(t *testing.T) *servertest.Server {
	t.Helper()

	s := servertest.New(t)
	at := s.Clock().Now()

	for i, p := range []*projects.Project{
		{ID: "p-shop", Name: "Shop", Description: "The new storefront", Owner: "Olive Owner", Tags: []string{"web", "launch"}},
		{ID: "p-blog", Name: "Blog", Description: "Posts", Owner: "Max Member", Status: "paused", Tags: []string{"web"}},
		{ID: "p-atlas", Name: "Atlas", Description: "Maps", Owner: "Kim", Tags: []string{}},
	} {
		p.Workspace = servertest.Acme
		p.UpdatedAt = at.Add(time.Duration(i) * time.Hour)
		s.Projects.Add(p)
	}

	return s
}

func TestProjects_AreTheCallersWorkspacesNewestFirst(t *testing.T) {
	t.Parallel()

	s := acme(t)
	p := list(t, s, "", s.As(servertest.Viewer))

	assert.Equal(t, []string{"Atlas", "Blog", "Shop"}, names(p), "Vault is Initrode's, which the viewer isn't in")
	assert.Equal(t, 3, p.Total)
	assert.Empty(t, p.Next, "the last page says no next")
}

func TestProjects_FilterSortAndPageAsTheClientAsks(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)

	assert.Equal(t, []string{"Blog", "Shop"}, names(list(t, s, "?tags=web", me)))
	assert.Equal(t, []string{"Shop"}, names(list(t, s, "?tags=web,launch", me)), "every tag asked")
	assert.Equal(t, []string{"Blog"}, names(list(t, s, "?status=paused", me)))
	assert.Equal(t, []string{"Shop"}, names(list(t, s, "?q=STOREFRONT", me)), "words in the description, any case")
	assert.Equal(t, []string{"Atlas"}, names(list(t, s, "?q=kim", me)), "or the owner")
	assert.Empty(t, names(list(t, s, "?q=%25", me)), "what was typed is never a pattern")
	assert.Equal(t, []string{"Atlas", "Blog", "Shop"}, names(list(t, s, "?sort=name:asc", me)))
	assert.Equal(t, []string{"Shop", "Blog", "Atlas"}, names(list(t, s, "?sort=name:desc", me)))
	assert.Equal(t, []string{"Shop", "Blog", "Atlas"}, names(list(t, s, "?sort=owner:desc", me)), "Olive, Max, Kim")

	first := list(t, s, "?sort=name:asc&limit=2", me)
	assert.Equal(t, []string{"Atlas", "Blog"}, names(first))
	assert.Equal(t, 3, first.Total, "the total is every match, not the page")
	require.Equal(t, "2", first.Next)

	assert.Equal(t, []string{"Shop"}, names(list(t, s, "?sort=name:asc&limit=2&cursor="+first.Next, me)))
}

func TestProjects_ACursorThatIsntOneStartsAtTheTop(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Member)

	for _, cursor := range []string{"abc", "-1", "99999999999"} {
		assert.Equal(t, []string{"Atlas", "Blog"}, names(list(t, s, "?sort=name:asc&limit=2&cursor="+cursor, me)), cursor)
	}
}

func TestProjects_APageOfMoreThanTheMostIsRefused(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodGet, fmt.Sprintf("/projects?limit=%d", pages.Max+1), "", s.As(servertest.Member))

	assert.Equal(t, http.StatusUnprocessableEntity, a.Status)
	assert.Contains(t, string(a.Body), `"field":"limit"`)
}

func TestProjects_AnotherTeamsWorkspaceAndProjectAreNotThere(t *testing.T) {
	t.Parallel()

	s := acme(t)
	me := s.As(servertest.Owner)

	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/projects?workspace="+servertest.Initrode, "", me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/projects/"+servertest.Vault, "", me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodPatch, "/projects/"+servertest.Vault, `{"name": "Mine"}`, me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodDelete, "/projects/"+servertest.Vault, "", me).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodPost, "/projects", `{"name": "X", "description": "", "workspace": "`+servertest.Initrode+`"}`, me).Status)
}

func TestProject_IsOneOfTheCallers(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodGet, "/projects/p-shop", "", s.As(servertest.Viewer))

	require.Equal(t, http.StatusOK, a.Status)
	assert.JSONEq(t, `{"id": "p-shop", "workspace": "w-acme", "name": "Shop", "description": "The new storefront", "status": "active",
		"owner": "Olive Owner", "tags": ["web", "launch"], "updatedAt": "2026-11-04T09:00:00Z"}`, string(a.Body))
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/projects/p-none", "", s.As(servertest.Viewer)).Status)
}

func TestCreateProject_IsTheMakersInTheirFirstWorkspace(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodPost, "/projects", `{"name": " Docs ", "description": "The handbook"}`, s.As(servertest.Member))

	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	var made struct {
		Workspace string   `json:"workspace"`
		Name      string   `json:"name"`
		Owner     string   `json:"owner"`
		Status    string   `json:"status"`
		Tags      []string `json:"tags"`
	}
	require.NoError(t, json.Unmarshal(a.Body, &made))
	assert.Equal(t, servertest.Acme, made.Workspace)
	assert.Equal(t, "Docs", made.Name, "trimmed")
	assert.Equal(t, "Max Member", made.Owner)
	assert.Equal(t, "active", made.Status)
	assert.Empty(t, made.Tags)
}

func TestCreateProject_ANameTakenOrEmptyIsRefusedOnTheField(t *testing.T) {
	t.Parallel()

	s := acme(t)

	for body, words := range map[string]string{
		`{"name": "shop", "description": ""}`: "Another project is already called shop.",
		`{"name": "  ", "description": ""}`:   "A project needs a name.",
	} {
		a := s.Do(http.MethodPost, "/projects", body, s.As(servertest.Member))

		assert.Equal(t, http.StatusUnprocessableEntity, a.Status, body)
		assert.JSONEq(t, `{"error": "`+words+`", "code": "invalid", "field": "name"}`, string(a.Body))
	}
}

func TestUpdateProject_ChangesWhatItSendsAndKeepsTheRest(t *testing.T) {
	t.Parallel()

	s := acme(t)
	s.Clock().Advance(time.Hour)
	me := s.As(servertest.Member)

	a := s.Do(http.MethodPatch, "/projects/p-shop", `{"status": "archived", "tags": [" web ", "web", "shop"]}`, me)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))
	assert.JSONEq(t, `{"id": "p-shop", "workspace": "w-acme", "name": "Shop", "description": "The new storefront", "status": "archived",
		"owner": "Olive Owner", "tags": ["web", "shop"], "updatedAt": "2026-11-04T10:00:00Z"}`, string(a.Body))

	renamed := s.Do(http.MethodPatch, "/projects/p-shop", `{"name": "SHOP"}`, me)
	assert.Equal(t, http.StatusOK, renamed.Status, "its own name, in another case, is no clash")

	assert.Equal(t, http.StatusUnprocessableEntity, s.Do(http.MethodPatch, "/projects/p-shop", `{"name": "Blog"}`, me).Status)
}

func TestProjects_EachRoleMayDoWhatTheClientsTableSays(t *testing.T) {
	t.Parallel()

	s := acme(t)
	viewer, member, owner := s.As(servertest.Viewer), s.As(servertest.Member), s.As(servertest.Owner)

	assert.Equal(t, http.StatusForbidden, s.Do(http.MethodPost, "/projects", `{"name": "X", "description": ""}`, viewer).Status)
	assert.Equal(t, http.StatusForbidden, s.Do(http.MethodPatch, "/projects/p-shop", `{"name": "X"}`, viewer).Status)
	assert.Equal(t, http.StatusForbidden, s.Do(http.MethodDelete, "/projects/p-shop", "", member).Status, "only an owner deletes")

	assert.Equal(t, http.StatusNoContent, s.Do(http.MethodDelete, "/projects/p-shop", "", owner).Status)
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/projects/p-shop", "", owner).Status)
}

func TestProjectTags_AreEveryTagInUseInTheCallersWorkspaces(t *testing.T) {
	t.Parallel()

	s := acme(t)
	a := s.Do(http.MethodGet, "/project-tags", "", s.As(servertest.Viewer))

	require.Equal(t, http.StatusOK, a.Status)
	assert.JSONEq(t, `["launch", "web"]`, string(a.Body))
	assert.Equal(t, http.StatusNotFound, s.Do(http.MethodGet, "/project-tags?workspace="+servertest.Initrode, "", s.As(servertest.Viewer)).Status)
}
