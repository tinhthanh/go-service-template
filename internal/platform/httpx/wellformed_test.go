package httpx_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tinhthanh/go-service-template/internal/platform/httpx"
)

// wellFormed sends a body through WellFormed and answers its status, the field refused (if any), and the body the handler saw.
func wellFormed(t *testing.T, contentType, body string, limit int64) (int, string, string) {
	t.Helper()

	var got string

	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)

		w.WriteHeader(http.StatusNoContent)
	}), httpx.MaxBody(limit), httpx.WellFormed(slog.New(slog.DiscardHandler)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var refused struct {
		Code  string `json:"code"`
		Field string `json:"field"`
	}

	if rec.Code != http.StatusNoContent {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refused), rec.Body.String())
	}

	return rec.Code, refused.Field, got
}

func TestWellFormed_AWellFormedBodyReachesTheHandlerAsItWasSent(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{"name": "Café 😀", "tags": ["a", "b"]}`, `{"name": "😀"}`, `[1, 2]`, "  {}\n", ""} {
		status, _, got := wellFormed(t, "application/json", body, 1<<20)
		assert.Equal(t, http.StatusNoContent, status, body)
		assert.Equal(t, body, got, "the body goes on unchanged")
	}
}

func TestWellFormed_TextThatIsntUTF8IsRefusedOnItsFieldNotKeptWithReplacementCharacters(t *testing.T) {
	t.Parallel()

	for _, body := range []string{"{\"name\": \"A\xff\xfeB\"}", `{"name": "\ud800"}`, "{\"name\": {\"inner\": \"\xc3\"}}"} {
		status, field, _ := wellFormed(t, "application/json; charset=utf-8", body, 1<<20)
		assert.Equal(t, http.StatusBadRequest, status, body)
		assert.Equal(t, "name", field, "the top-level field it broke in")
	}
}

func TestWellFormed_ANameSentTwiceIsRefusedRatherThanTheLastOneKept(t *testing.T) {
	t.Parallel()

	status, field, _ := wellFormed(t, "application/json", `{"name": "Shop", "name": "Other"}`, 1<<20)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "name", field)
}

func TestWellFormed_AnythingAfterTheValueIsRefused(t *testing.T) {
	t.Parallel()

	status, field, _ := wellFormed(t, "application/json", `{"name": "Shop"} {"name": "Other"}`, 1<<20)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Empty(t, field, "it is about no field")
}

func TestWellFormed_ABodyThatIsntJSONGoesOnForTheContractToJudge(t *testing.T) {
	t.Parallel()

	status, _, got := wellFormed(t, "text/plain", "A\xffB", 1<<20)
	assert.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, "A\xffB", got)

	status, _, _ = wellFormed(t, "application/problem+json", `{"a": 1, "a": 2}`, 1<<20)
	assert.Equal(t, http.StatusBadRequest, status, "a +json type is JSON too")
}

func TestWellFormed_ABodyPastMaxBodyIsTooLarge(t *testing.T) {
	t.Parallel()

	status, _, _ := wellFormed(t, "application/json", `{"name": "`+strings.Repeat("x", 64)+`"}`, 16)
	assert.Equal(t, http.StatusRequestEntityTooLarge, status)
}
