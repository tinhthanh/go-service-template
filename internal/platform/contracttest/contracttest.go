// Package contracttest holds a test's answers to the client's contract:
// every response a test reads is checked against the operation's own
// response schema (its status, or default), so an answer of the wrong
// shape fails the test that made it. Tests only.
package contracttest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/tinhthanh/go-service-template/internal/api"
)

// Client calls a handler mounted at prefix and checks every answer against the contract.
type Client struct {
	t       *testing.T
	handler http.Handler
	prefix  string
	router  routers.Router
}

// New -.
func New(t *testing.T, handler http.Handler, prefix string) *Client {
	t.Helper()

	spec, err := api.GetSpec()
	if err != nil {
		t.Fatalf("contracttest: the contract: %v", err)
	}

	spec.Servers = openapi3.Servers{{URL: prefix}}

	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("contracttest: a router for the contract: %v", err)
	}

	return &Client{t: t, handler: handler, prefix: prefix, router: router}
}

// Answer is what a call got: its status, headers and body.
type Answer struct {
	Status int
	Header http.Header
	Body   []byte
}

// Cookies are the cookies the answer set.
func (a *Answer) Cookies() []*http.Cookie {
	return (&http.Response{Header: a.Header}).Cookies()
}

// Do sends the request (a path under the prefix) and checks the answer.
// A write carries X-Requested-With, as the client's own calls do.
func (c *Client) Do(method, path, body string, header map[string]string) Answer {
	c.t.Helper()

	req := httptest.NewRequestWithContext(c.t.Context(), method, c.prefix+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if method != http.MethodGet {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	for k, v := range header {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)

	a := Answer{Status: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
	c.check(req, &a)

	return a
}

func (c *Client) check(req *http.Request, a *Answer) {
	c.t.Helper()

	route, params, err := c.router.FindRoute(req)
	if err != nil {
		c.t.Fatalf("contracttest: %s %s is not an operation of the contract: %v", req.Method, req.URL.Path, err)
	}

	in := &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: route,
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}

	out := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in,
		Status:                 a.Status,
		Header:                 a.Header,
		Body:                   io.NopCloser(bytes.NewReader(a.Body)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}

	if err := openapi3filter.ValidateResponse(c.t.Context(), out); err != nil {
		c.t.Errorf("contracttest: %s %s answered %d, which the contract refuses: %v\n%s", req.Method, req.URL.Path, a.Status, err, a.Body)
	}
}
