package workspaces_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/server/servertest"
)

func TestWorkspaces_AreThoseThePersonBelongsTo(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	a := s.Do(http.MethodGet, "/workspaces", "", s.As(servertest.Member))

	require.Equal(t, http.StatusOK, a.Status)
	assert.JSONEq(t, `[{"id": "w-acme", "name": "Acme"}]`, string(a.Body), "not Initrode, which is somebody else's")
}

func TestWorkspaces_NobodySignedInIs401(t *testing.T) {
	t.Parallel()

	assert.Equal(t, http.StatusUnauthorized, servertest.New(t).Do(http.MethodGet, "/workspaces", "", nil).Status)
}

func TestCreateWorkspace_ItsMakerBelongsToItAtOnce(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	me := s.As(servertest.Member)

	made := s.Do(http.MethodPost, "/workspaces", `{"name": "  Acme Labs "}`, me)
	require.Equal(t, http.StatusOK, made.Status, string(made.Body))
	assert.JSONEq(t, `{"id": "w-acme-labs", "name": "Acme Labs"}`, string(made.Body))

	assert.JSONEq(t, `[{"id": "w-acme", "name": "Acme"}, {"id": "w-acme-labs", "name": "Acme Labs"}]`, string(s.Do(http.MethodGet, "/workspaces", "", me).Body))
	assert.Equal(t, http.StatusOK, s.Do(http.MethodPost, "/projects", `{"name": "Lab", "description": "", "workspace": "w-acme-labs"}`, me).Status, "a project may be made there on the next call")
}

func TestCreateWorkspace_AnIDTakenGetsANumber(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	// Initrode is taken by a workspace the owner isn't in, so the name is theirs to use.
	made := s.Do(http.MethodPost, "/workspaces", `{"name": "Initrode"}`, s.As(servertest.Owner))

	require.Equal(t, http.StatusOK, made.Status, string(made.Body))
	assert.JSONEq(t, `{"id": "w-initrode-2", "name": "Initrode"}`, string(made.Body))
}

func TestCreateWorkspace_ANameTheyAlreadyHaveIsRefusedOnTheField(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	for body, words := range map[string]string{
		`{"name": "acme"}`: "You're already in a workspace called Acme.",
		`{"name": "   "}`:  "A workspace needs a name.",
	} {
		a := s.Do(http.MethodPost, "/workspaces", body, s.As(servertest.Member))

		assert.Equal(t, http.StatusUnprocessableEntity, a.Status, body)
		assert.JSONEq(t, `{"error": "`+words+`", "code": "invalid", "field": "name"}`, string(a.Body))
	}
}

func TestCreateWorkspace_AViewerMayNot(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	assert.Equal(t, http.StatusForbidden, s.Do(http.MethodPost, "/workspaces", `{"name": "Mine"}`, s.As(servertest.Viewer)).Status)
}
