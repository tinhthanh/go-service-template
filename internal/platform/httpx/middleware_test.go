package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tinhthanh/go-service-template/internal/platform/httpx"
)

func serve(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func get(t *testing.T, header map[string]string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x?q=secret", http.NoBody)
	for k, v := range header {
		req.Header.Set(k, v)
	}

	return req
}

// teapot is a handler that answers, so a test sees the middleware around it.
func teapot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
}

func TestRecover_APanicIsA500InTheContractsShape(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := serve(t, httpx.Recover(slog.New(slog.NewTextHandler(&logs, nil)))(boom), get(t, nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error": "Something went wrong on the server.", "code": "internal"}`, rec.Body.String())
	assert.Contains(t, logs.String(), "boom")
}

func TestLog_OneLinePerRequestWithoutTheQuery(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	h := httpx.Chain(teapot(), httpx.RequestID(), httpx.Log(slog.New(slog.NewTextHandler(&logs, nil))))
	serve(t, h, get(t, map[string]string{"X-Request-ID": "r-1"}))

	assert.Equal(t, 1, strings.Count(logs.String(), "\n"))
	assert.Contains(t, logs.String(), "status=418")
	assert.Contains(t, logs.String(), "request_id=r-1")
	assert.NotContains(t, logs.String(), "secret", "the query can carry what a person typed")
}

func TestRequestID_AnIDTooLongIsReplaced(t *testing.T) {
	t.Parallel()

	rec := serve(t, httpx.RequestID()(teapot()), get(t, map[string]string{"X-Request-ID": strings.Repeat("x", 65)}))

	assert.Len(t, rec.Header().Get("X-Request-ID"), 16)
}

func TestCORS_APreflightFromTheClientIsAnswered(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/x", http.NoBody)
	req.Header.Set("Origin", "https://app.test")
	req.Header.Set("Access-Control-Request-Method", http.MethodPatch)

	rec := serve(t, httpx.CORS([]string{"https://app.test"})(teapot()), req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "X-Requested-With")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), http.MethodPatch)
}

func TestChain_TheFirstMiddlewareIsOutermost(t *testing.T) {
	t.Parallel()

	var order []string

	mark := func(name string) httpx.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	serve(t, httpx.Chain(teapot(), mark("a"), mark("b")), get(t, nil))

	assert.Equal(t, []string{"a", "b"}, order)
}
