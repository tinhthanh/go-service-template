package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tinhthanh/go-service-template/internal/platform/httpx"
)

// seen is what the handler behind Nulls saw: the fields sent as null, of those asked, and the body.
func seen(t *testing.T, method, body string, fields ...string) ([]string, string) {
	t.Helper()

	var nulls []string

	var got string

	h := httpx.Nulls()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		for _, f := range fields {
			if httpx.SentNull(r.Context(), f) {
				nulls = append(nulls, f)
			}
		}

		b, _ := io.ReadAll(r.Body)
		got = string(b)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), method, "/x", strings.NewReader(body)))

	return nulls, got
}

func TestNulls_APatchSaysWhichFieldsItSentAsNullAndKeepsItsBody(t *testing.T) {
	t.Parallel()

	body := `{"due": null, "project": "p-1", "title": "x"}`
	nulls, got := seen(t, http.MethodPatch, body, "due", "project", "title", "attachment")

	assert.Equal(t, []string{"due"}, nulls, "a value set and a field left out are not null")
	assert.Equal(t, body, got, "the body reaches the handler as it was sent")
}

func TestNulls_OnlyAPatchWithAJSONObjectIsRead(t *testing.T) {
	t.Parallel()

	nulls, _ := seen(t, http.MethodPost, `{"due": null}`, "due")
	assert.Empty(t, nulls, "a POST's null is the same as leaving it out")

	nulls, got := seen(t, http.MethodPatch, `[null`, "due")
	assert.Empty(t, nulls)
	assert.Equal(t, `[null`, got, "a body that isn't an object goes on for the contract to refuse")
}
