// Package server puts the service together: the client's contract under
// APIPrefix, each operation answered by the feature that builds it or by
// api.Unimplemented (a 501), every request checked against the contract
// first, all behind httpx's middleware; and /healthz beside it.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/tinhthanh/go-service-template/internal/api"
	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/config"
	"github.com/tinhthanh/go-service-template/internal/platform/httpx"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/session"
)

// operations is every operation of the contract. A feature's handler,
// embedded here, answers its operations; stubs sits one level deeper, so a
// feature's method wins over the stub, and two features answering the same
// operation fail to compile (an ambiguous selector).
type operations struct {
	*session.Handler
	stubs
}

type stubs struct{ api.Unimplemented }

var _ api.StrictServerInterface = (*operations)(nil)

// Features are the features' handlers, and the session that names callers.
type Features struct {
	Auth    auth.Authenticator
	Session *session.Handler
}

// New is the service's handler.
func New(cfg *config.Config, log *slog.Logger, f Features) (http.Handler, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("server: the contract: %w", err)
	}

	// The router matches paths without the prefix; the validator does too.
	spec.Servers = nil

	strict := api.NewStrictHandlerWithOptions(&operations{Handler: f.Session}, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  func(w http.ResponseWriter, r *http.Request, err error) { problem.Write(w, r, log, invalid(err)) },
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) { problem.Write(w, r, log, err) },
	})

	validate := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		Options:               openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		DoNotValidateServers:  true,
		SilenceServersWarning: true,
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, _ nethttpmiddleware.ErrorHandlerOpts) {
			problem.Write(w, r, log, invalid(err))
		},
	})

	contract := http.NewServeMux()
	api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseRouter:       contract,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) { problem.Write(w, r, log, invalid(err)) },
	})

	mux := http.NewServeMux()
	// A write needs the app's header (CSRF), then the caller is found, then the request is held to the contract.
	guarded := httpx.Chain(validate(contract), auth.CSRF(log), auth.Middleware(f.Auth, log))
	mux.Handle(cfg.APIPrefix+"/", http.StripPrefix(cfg.APIPrefix, guarded))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	return httpx.Chain(mux,
		httpx.RequestID(),
		httpx.Log(log),
		httpx.Recover(log),
		httpx.SecurityHeaders(),
		httpx.CORS(cfg.CORSOrigins),
	), nil
}

// fieldInReason reads the field a JSON Schema refusal is about from
// kin-openapi's reason (an OpenAPI 3.1 document is checked by
// santhosh-tekuri/jsonschema): "missing property 'x'", "additional
// properties 'x' not allowed", or "at '/x': …".
var fieldInReason = regexp.MustCompile(`(?:missing propert(?:y|ies)|additional propert(?:y|ies)) '([^']+)'|at '/([^/']+)`)

// invalid is a request the contract refuses, on the field it names when it names one.
func invalid(err error) error {
	reqErr, ok := errors.AsType[*openapi3filter.RequestError](err)
	if !ok {
		if errors.Is(err, problem.ErrInvalid) {
			return err
		}

		return fmt.Errorf("%w: %w", problem.ErrInvalid, err)
	}

	if reqErr.Parameter != nil {
		return problem.Malformed(reqErr.Parameter.Name, "This value isn't accepted here.")
	}

	if schemaErr, isSchema := reqErr.Err.(*openapi3.SchemaError); isSchema { //nolint:errorlint // RequestError keeps it in Err, unwrapped
		if m := fieldInReason.FindStringSubmatch(schemaErr.Reason); m != nil {
			field := m[1] + m[2]
			if schemaErr.SchemaField == "required" {
				return problem.Malformed(field, "This is needed.")
			}

			return problem.Malformed(field, "This value isn't accepted here.")
		}
	}

	return fmt.Errorf("%w: %w", problem.ErrInvalid, err)
}
