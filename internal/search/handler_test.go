package search_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/server/servertest"
)

type hit struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

// acme is a server with projects and tasks, made through the server as the client would.
func acme(t *testing.T) (*servertest.Server, map[string]string) {
	t.Helper()

	s := servertest.New(t)
	me := s.As(servertest.Member)
	ids := map[string]string{}

	for _, p := range []string{"Shop", "Shopfront redesign", "Workshop", "Blog"} {
		a := s.Do(http.MethodPost, "/projects", `{"name": "`+p+`", "description": "A shop-ish thing"}`, me)
		require.Equal(t, http.StatusOK, a.Status, string(a.Body))

		var made struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(a.Body, &made))

		ids[p] = made.ID
	}

	for _, title := range []string{"Price the shop", "Write the post"} {
		body := `{"title": "` + title + `", "project": "` + ids["Blog"] + `"}`
		require.Equal(t, http.StatusOK, s.Do(http.MethodPost, "/tasks", body, me).Status)
	}

	return s, ids
}

func find(t *testing.T, s *servertest.Server, q string, who map[string]string) []hit {
	t.Helper()

	a := s.Do(http.MethodGet, "/search?q="+url.QueryEscape(q), "", who)
	require.Equal(t, http.StatusOK, a.Status, string(a.Body))

	var hits []hit
	require.NoError(t, json.Unmarshal(a.Body, &hits))

	return hits
}

func names(hits []hit) []string {
	out := []string{}
	for _, h := range hits {
		out = append(out, h.Kind+":"+h.Name)
	}

	return out
}

func TestSearch_RanksAsTheClientDoesEachKindOnItsOwn(t *testing.T) {
	t.Parallel()

	s, _ := acme(t)

	// The name itself, then one that starts with it, then a word that does, then the name has it;
	// the descriptions all have it too, but a name match comes first. Then the tasks.
	assert.Equal(t, []string{
		"project:Shop", "project:Shopfront redesign", "project:Workshop", "project:Blog",
		"task:Price the shop",
	}, names(find(t, s, "shop", s.As(servertest.Viewer))))
}

func TestSearch_AHitSaysItsDetailAndStatus(t *testing.T) {
	t.Parallel()

	s, ids := acme(t)
	hits := find(t, s, "post", s.As(servertest.Viewer))

	require.Len(t, hits, 1)
	assert.Equal(t, hit{Kind: "task", ID: hits[0].ID, Name: "Write the post", Detail: "Blog", Status: "todo"}, hits[0], "a task's detail is its project's name")

	project := find(t, s, "blog", s.As(servertest.Viewer))[0]
	assert.Equal(t, hit{Kind: "project", ID: ids["Blog"], Name: "Blog", Detail: "Max Member", Status: "active"}, project, "a project's is its owner")
}

func TestSearch_FindsOnlyTheCallersWorkspaces(t *testing.T) {
	t.Parallel()

	s, _ := acme(t)

	assert.Empty(t, find(t, s, "vault", s.As(servertest.Owner)), "Vault is Initrode's")
}

func TestSearch_NoLettersFindNothingAndTypedIsNeverAPattern(t *testing.T) {
	t.Parallel()

	s, _ := acme(t)
	me := s.As(servertest.Viewer)

	assert.Empty(t, find(t, s, "   ", me))
	assert.Empty(t, find(t, s, "%", me))
	assert.Empty(t, find(t, s, ".*", me))
}

func TestSearch_NobodySignedInIs401(t *testing.T) {
	t.Parallel()

	assert.Equal(t, http.StatusUnauthorized, servertest.New(t).Do(http.MethodGet, "/search?q=shop", "", nil).Status)
}
