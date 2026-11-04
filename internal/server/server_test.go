package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/contracttest"
	"github.com/tinhthanh/go-service-template/internal/server/servertest"
)

const _origin = "https://app.acme.test"

func client(t *testing.T) *contracttest.Client {
	t.Helper()

	return servertest.New(t).Client
}

func code(t *testing.T, a contracttest.Answer) string {
	t.Helper()

	var body struct {
		Code  string `json:"code"`
		Field string `json:"field"`
	}
	require.NoError(t, json.Unmarshal(a.Body, &body), string(a.Body))

	return body.Code
}

func TestServer_AnOperationNotBuiltSaysSoInTheContractsWords(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)
	owner := s.As(servertest.Owner)

	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, "/tasks", ""},
		{http.MethodGet, "/tasks/t-1", ""},
		{http.MethodDelete, "/tasks/t-1", ""},
	} {
		a := s.Do(call.method, call.path, call.body, owner)

		assert.Equal(t, http.StatusNotImplemented, a.Status, "%s %s", call.method, call.path)
		assert.Equal(t, "unavailable", code(t, a))
	}
}

// The contract's x-permission is checked before any handler, built or not: a
// feature can't forget it, and one not built yet already refuses as it will.
func TestServer_EveryOperationsXPermissionIsCheckedFirst(t *testing.T) {
	t.Parallel()

	s := servertest.New(t)

	assert.Equal(t, "unauthorized", code(t, s.Do(http.MethodGet, "/tasks", "", nil)), "signed-in: nobody is 401")
	assert.Equal(t, "forbidden", code(t, s.Do(http.MethodDelete, "/tasks/t-1", "", s.As(servertest.Member))), "task.delete is an owner's")
	assert.Equal(t, "forbidden", code(t, s.Do(http.MethodPost, "/tasks", `{"title": "Plan"}`, s.As(servertest.Viewer))), "task.create isn't a viewer's")
	assert.Equal(t, http.StatusOK, s.Do(http.MethodPost, "/session", `{"email": "viewer@acme.test", "password": "`+servertest.Password+`"}`, nil).Status, "public: anyone")
}

func TestServer_ARequestTheContractRefusesIsInvalidOnItsField(t *testing.T) {
	t.Parallel()

	c := client(t)

	missing := c.Do(http.MethodPost, "/projects", `{"name": "Shop"}`, nil)
	assert.Equal(t, http.StatusBadRequest, missing.Status)
	assert.Equal(t, "invalid", code(t, missing))
	assert.Contains(t, string(missing.Body), `"field":"description"`)

	extra := c.Do(http.MethodPost, "/projects", `{"name": "Shop", "description": "x", "owner": "me"}`, nil)
	assert.Equal(t, http.StatusBadRequest, extra.Status, "a field the contract doesn't have")
	assert.Contains(t, string(extra.Body), `"field":"owner"`)

	wrong := c.Do(http.MethodPost, "/projects", `{"name": 7, "description": "x"}`, nil)
	assert.Contains(t, string(wrong.Body), `"field":"name"`, "a value of the wrong type")

	day := c.Do(http.MethodPost, "/tasks", `{"title": "X", "due": "tomorrow"}`, nil)
	assert.Equal(t, http.StatusBadRequest, day.Status)
	assert.Contains(t, string(day.Body), `"field":"due"`, "a value none of anyOf's schemas takes (a day, or null)")

	status := c.Do(http.MethodGet, "/projects?status=sleeping", "", nil)
	assert.Equal(t, http.StatusBadRequest, status.Status)
	assert.Contains(t, string(status.Body), `"field":"status"`)
}

func TestServer_EveryAnswerCarriesARequestIDAndSecurityHeaders(t *testing.T) {
	t.Parallel()

	c := client(t)

	given := c.Do(http.MethodGet, "/tasks", "", map[string]string{"X-Request-ID": "abc123"})
	assert.Equal(t, "abc123", given.Header.Get("X-Request-ID"), "the caller's id is kept")

	made := c.Do(http.MethodGet, "/tasks", "", nil)
	assert.Len(t, made.Header.Get("X-Request-ID"), 16)
	assert.Equal(t, "nosniff", made.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "no-store", made.Header.Get("Cache-Control"))
}

func TestServer_OnlyTheClientsOriginMayCallWithItsCookies(t *testing.T) {
	t.Parallel()

	c := client(t)

	ours := c.Do(http.MethodGet, "/tasks", "", map[string]string{"Origin": _origin})
	assert.Equal(t, _origin, ours.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", ours.Header.Get("Access-Control-Allow-Credentials"))

	other := c.Do(http.MethodGet, "/tasks", "", map[string]string{"Origin": "https://elsewhere.test"})
	assert.Empty(t, other.Header.Get("Access-Control-Allow-Origin"))
}
