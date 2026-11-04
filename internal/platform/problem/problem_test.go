package problem_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

var errDisk = errors.New("disk on fire")

func TestOf_EachRefusalHasItsStatusAndCode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{problem.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
		{problem.ErrForbidden, http.StatusForbidden, "forbidden"},
		{problem.ErrNotFound, http.StatusNotFound, "not_found"},
		{problem.ErrConflict, http.StatusConflict, "conflict"},
		{problem.ErrInvalid, http.StatusBadRequest, "invalid"},
		{problem.ErrNotBuilt, http.StatusNotImplemented, "unavailable"},
		{fmt.Errorf("projects.Get: %w", problem.ErrNotFound), http.StatusNotFound, "not_found"},
		{errDisk, http.StatusInternalServerError, "internal"},
	} {
		status, body := problem.Of(tc.err)

		assert.Equal(t, tc.status, status, tc.err)
		assert.Equal(t, tc.code, body.Code, tc.err)
		assert.NotEmpty(t, body.Error)
	}
}

func TestOf_AFieldRefusalNamesItsField(t *testing.T) {
	t.Parallel()

	status, body := problem.Of(fmt.Errorf("create: %w", problem.Invalid("name", "A project needs a name.")))

	assert.Equal(t, http.StatusUnprocessableEntity, status, "a value the rules refuse, as the client's mock answers it")
	assert.Equal(t, problem.Body{Error: "A project needs a name.", Code: "invalid", Field: "name"}, body)
	require.ErrorIs(t, problem.Invalid("name", "x"), problem.ErrInvalid)

	status, body = problem.Of(problem.Malformed("status", "This value isn't accepted here."))
	assert.Equal(t, http.StatusBadRequest, status, "a request the contract refuses")
	assert.Equal(t, "status", body.Field)
}

func TestWrite_TheRawErrorIsLoggedNeverSent(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	rec := httptest.NewRecorder()
	problem.Write(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody), slog.New(slog.NewTextHandler(&logs, nil)), errDisk)

	var body problem.Body
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.NotContains(t, rec.Body.String(), "disk on fire")
	assert.Contains(t, logs.String(), "disk on fire")
}
