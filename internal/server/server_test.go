package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/config"
	"github.com/tinhthanh/go-service-template/internal/platform/contracttest"
	"github.com/tinhthanh/go-service-template/internal/server"
)

const _origin = "https://app.example.test"

func client(t *testing.T) *contracttest.Client {
	t.Helper()

	cfg := &config.Config{APIPrefix: "/api", CORSOrigins: []string{_origin}}

	h, err := server.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	return contracttest.New(t, h, "/api")
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

	c := client(t)

	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, "/projects", ""},
		{http.MethodGet, "/projects/p-1", ""},
		{http.MethodPost, "/projects", `{"name": "Shop", "description": "A new shop"}`},
		{http.MethodGet, "/session", ""},
	} {
		a := c.Do(call.method, call.path, call.body, nil)

		assert.Equal(t, http.StatusNotImplemented, a.Status, "%s %s", call.method, call.path)
		assert.Equal(t, "unavailable", code(t, a))
	}
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

	status := c.Do(http.MethodGet, "/projects?status=sleeping", "", nil)
	assert.Equal(t, http.StatusBadRequest, status.Status)
	assert.Contains(t, string(status.Body), `"field":"status"`)
}

func TestServer_EveryAnswerCarriesARequestIDAndSecurityHeaders(t *testing.T) {
	t.Parallel()

	c := client(t)

	given := c.Do(http.MethodGet, "/projects", "", map[string]string{"X-Request-ID": "abc123"})
	assert.Equal(t, "abc123", given.Header.Get("X-Request-ID"), "the caller's id is kept")

	made := c.Do(http.MethodGet, "/projects", "", nil)
	assert.Len(t, made.Header.Get("X-Request-ID"), 16)
	assert.Equal(t, "nosniff", made.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "no-store", made.Header.Get("Cache-Control"))
}

func TestServer_OnlyTheClientsOriginMayCallWithItsCookies(t *testing.T) {
	t.Parallel()

	c := client(t)

	ours := c.Do(http.MethodGet, "/projects", "", map[string]string{"Origin": _origin})
	assert.Equal(t, _origin, ours.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", ours.Header.Get("Access-Control-Allow-Credentials"))

	other := c.Do(http.MethodGet, "/projects", "", map[string]string{"Origin": "https://elsewhere.test"})
	assert.Empty(t, other.Header.Get("Access-Control-Allow-Origin"))
}
