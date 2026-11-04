package auth_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

// rules reads a contract of three operations: one public, one for the
// signed-in, one for an action only an owner takes.
func rules(t *testing.T, extra string) (*auth.Rules, error) {
	t.Helper()

	spec, err := openapi3.NewLoader().LoadFromData([]byte(`{"openapi": "3.1.0", "info": {"title": "x", "version": "1"},
	  "x-roles": {"project.delete": ["owner"]` + extra + `},
	  "paths": {
	    "/session": {"post": {"operationId": "signIn", "x-permission": "public", "responses": {"204": {"description": "x"}}}},
	    "/projects": {"get": {"operationId": "projects", "x-permission": "signed-in", "responses": {"204": {"description": "x"}}}},
	    "/projects/{id}": {"delete": {"operationId": "deleteProject", "x-permission": "project.delete", "responses": {"204": {"description": "x"}}}}
	  }}`))
	require.NoError(t, err)

	return auth.RulesOf(spec)
}

func TestRules_AreReadFromTheContract(t *testing.T) {
	t.Parallel()

	r, err := rules(t, "")
	require.NoError(t, err)

	assert.Equal(t, "project.delete", r.Need("DeleteProject"), "the generated name, in any case")
	assert.True(t, r.Can(&auth.Principal{Role: "owner"}, "project.delete"))
	assert.False(t, r.Can(&auth.Principal{Role: "member"}, "project.delete"))
	assert.False(t, r.Can(&auth.Principal{Role: "owner"}, "project.launch"), "an action x-roles doesn't name is nobody's")
	assert.False(t, r.Can(nil, "project.delete"), "nobody may take none")
}

func TestRules_CheckIsTheOperationsXPermissionForTheCaller(t *testing.T) {
	t.Parallel()

	r, err := rules(t, "")
	require.NoError(t, err)

	owner := auth.With(t.Context(), &auth.Principal{Role: "owner"})
	viewer := auth.With(t.Context(), &auth.Principal{Role: "viewer"})

	require.NoError(t, r.Check(t.Context(), "SignIn"), "public: anyone")
	require.ErrorIs(t, r.Check(t.Context(), "Projects"), problem.ErrUnauthorized)
	require.NoError(t, r.Check(viewer, "Projects"), "signed-in: any role")
	require.ErrorIs(t, r.Check(viewer, "DeleteProject"), problem.ErrForbidden)
	require.NoError(t, r.Check(owner, "DeleteProject"))
}

func TestRulesOf_AContractThatDoesntSayIsRefusedAtStart(t *testing.T) {
	t.Parallel()

	spec, err := openapi3.NewLoader().LoadFromData([]byte(`{"openapi": "3.1.0", "info": {"title": "x", "version": "1"},
	  "paths": {"/x": {"get": {"operationId": "x", "responses": {"204": {"description": "x"}}}}}}`))
	require.NoError(t, err)

	_, err = auth.RulesOf(spec)
	require.ErrorIs(t, err, auth.ErrRules)
	require.ErrorContains(t, err, "no x-permission")

	spec, err = openapi3.NewLoader().LoadFromData([]byte(`{"openapi": "3.1.0", "info": {"title": "x", "version": "1"}, "x-roles": {},
	  "paths": {"/x": {"get": {"operationId": "x", "x-permission": "x.launch", "responses": {"204": {"description": "x"}}}}}}`))
	require.NoError(t, err)

	_, err = auth.RulesOf(spec)
	require.ErrorContains(t, err, "not an action of x-roles")
}

func TestRequired_NobodyIsUnauthorized(t *testing.T) {
	t.Parallel()

	_, err := auth.Required(t.Context())
	require.ErrorIs(t, err, problem.ErrUnauthorized)

	p, err := auth.Required(auth.With(t.Context(), &auth.Principal{ID: "u-1"}))
	require.NoError(t, err)
	assert.Equal(t, "u-1", p.ID)
}

type names map[string]auth.Principal

func (n names) Authenticate(_ context.Context, token string) (auth.Principal, error) {
	if p, ok := n[token]; ok {
		return p, nil
	}

	return auth.Principal{}, problem.ErrUnauthorized
}

// seen is who the handler behind the middleware saw, and the tokens.
func seen(t *testing.T, cookie string) (*auth.Principal, auth.Tokens) {
	t.Helper()

	var (
		who    *auth.Principal
		tokens auth.Tokens
	)

	h := auth.Middleware(names{"live": {ID: "u-1"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		who, _ = auth.From(r.Context())
		tokens = auth.TokensFrom(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	h.ServeHTTP(httptest.NewRecorder(), req)

	return who, tokens
}

func TestMiddleware_ALiveSessionNamesTheCaller(t *testing.T) {
	t.Parallel()

	who, tokens := seen(t, "session=live; refresh=r-1")

	require.NotNil(t, who)
	assert.Equal(t, "u-1", who.ID)
	assert.Equal(t, auth.Tokens{Session: "live", Refresh: "r-1"}, tokens)
}

func TestMiddleware_AnEndedSessionOrNoneGoesOnWithNobody(t *testing.T) {
	t.Parallel()

	who, _ := seen(t, "session=ended")
	assert.Nil(t, who)

	who, _ = seen(t, "")
	assert.Nil(t, who)
}

func TestCSRF_AWriteNeedsTheAppsHeader(t *testing.T) {
	t.Parallel()

	h := auth.CSRF(slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	status := func(method, header string) int {
		req := httptest.NewRequestWithContext(t.Context(), method, "/x", http.NoBody)
		if header != "" {
			req.Header.Set("X-Requested-With", header)
		}

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	assert.Equal(t, http.StatusNoContent, status(http.MethodGet, ""), "a read needs nothing")
	assert.Equal(t, http.StatusForbidden, status(http.MethodPost, ""))
	assert.Equal(t, http.StatusForbidden, status(http.MethodDelete, "fetch"))
	assert.Equal(t, http.StatusNoContent, status(http.MethodPatch, "XMLHttpRequest"))
}
