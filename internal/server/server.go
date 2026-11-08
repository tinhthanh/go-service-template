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
	"github.com/tinhthanh/go-service-template/internal/files"
	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/config"
	"github.com/tinhthanh/go-service-template/internal/platform/httpx"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
	"github.com/tinhthanh/go-service-template/internal/projects"
	"github.com/tinhthanh/go-service-template/internal/search"
	"github.com/tinhthanh/go-service-template/internal/session"
	"github.com/tinhthanh/go-service-template/internal/tasks"
	"github.com/tinhthanh/go-service-template/internal/workspaces"
)

// operations is every operation of the contract. A feature's handler,
// embedded here, answers its operations; stubs sits one level deeper, so a
// feature's method wins over the stub, and two features answering the same
// operation fail to compile (an ambiguous selector).
type operations struct {
	*sessionOps
	*workspaceOps
	*projectOps
	*taskOps
	*searchOps
	*fileOps
	stubs
}

// Each feature's handler is called Handler; an alias names its field here.
type (
	sessionOps   = session.Handler
	workspaceOps = workspaces.Handler
	projectOps   = projects.Handler
	taskOps      = tasks.Handler
	searchOps    = search.Handler
	fileOps      = files.Handler
)

type stubs struct{ api.Unimplemented }

var _ api.StrictServerInterface = (*operations)(nil)

// Features are the features' handlers, and the session that names callers.
type Features struct {
	Auth       auth.Authenticator
	Session    *session.Handler
	Workspaces *workspaces.Handler
	Projects   *projects.Handler
	Tasks      *tasks.Handler
	Search     *search.Handler
	Files      *files.Handler
}

// New is the service's handler.
func New(cfg *config.Config, log *slog.Logger, f Features) (http.Handler, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("server: the contract: %w", err)
	}

	// The router matches paths without the prefix; the validator does too.
	spec.Servers = nil

	ops := &operations{sessionOps: f.Session, workspaceOps: f.Workspaces, projectOps: f.Projects, taskOps: f.Tasks, searchOps: f.Search, fileOps: f.Files}

	// What each operation needs, and who may: the contract's x-permission and x-roles.
	rules, err := auth.RulesOf(spec)
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	strict := api.NewStrictHandlerWithOptions(ops, []api.StrictMiddlewareFunc{permitted(rules)}, api.StrictHTTPServerOptions{
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
	// A write needs the app's header (CSRF), then the caller is found, then a JSON body is held to RFC 8259
	// as sent (httpx.WellFormed: no text that isn't UTF-8, no name twice), then the fields a change sends as
	// null are noted (httpx.Nulls), then the request is held to the contract.
	guarded := httpx.Chain(validate(contract), httpx.MaxBody(maxBody), auth.CSRF(log), auth.Middleware(f.Auth, log), httpx.WellFormed(log), httpx.Nulls())
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

// maxBody is the most a request's body may hold: a file's 5 MB (internal/files) and room besides.
const maxBody = 8 << 20

// permitted checks an operation's x-permission before its handler, or its
// stub, runs: a role refused is 403 whether or not the operation is built.
func permitted(rules *auth.Rules) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operation string) api.StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
			if err := rules.Check(ctx, operation); err != nil {
				return nil, err
			}

			return next(ctx, w, r, req)
		}
	}
}

// fieldInReason reads the field a JSON Schema refusal is about from
// kin-openapi's reason (an OpenAPI 3.1 document is checked by
// santhosh-tekuri/jsonschema): "missing property 'x'", "additional
// properties 'x' not allowed", or "at '/x': …"; or, for a body with an anyOf
// in it, from kin-openapi's own: `property "x" is missing` or `… is unsupported`.
var fieldInReason = regexp.MustCompile(`(?:missing propert(?:y|ies)|additional propert(?:y|ies)) '([^']+)'|at '/([^/']+)|property "([^"]+)" is (?:missing|unsupported)`)

// invalid is a request the contract refuses, on the field it names when it names one.
func invalid(err error) error {
	reqErr, ok := errors.AsType[*openapi3filter.RequestError](err)
	if ok {
		// A body cut short by httpx.MaxBody is too large, not malformed; RequestError keeps it in Err.
		if _, big := errors.AsType[*http.MaxBytesError](reqErr.Err); big {
			return problem.ErrTooLarge
		}
	}

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
			field := m[1] + m[2] + m[3]
			if schemaErr.SchemaField == "required" {
				return problem.Malformed(field, "This is needed.")
			}

			return problem.Malformed(field, "This value isn't accepted here.")
		}

		// A reason that names no field (anyOf: a day that isn't one, where null is allowed too) is
		// still about the value at its pointer.
		if at := schemaErr.JSONPointer(); len(at) > 0 {
			return problem.Malformed(at[0], "This value isn't accepted here.")
		}
	}

	return fmt.Errorf("%w: %w", problem.ErrInvalid, err)
}
