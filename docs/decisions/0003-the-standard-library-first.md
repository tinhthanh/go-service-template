# 0003: net/http and log/slog, not a framework

**Status:** accepted, 2026-11-04

**Context.** Go 1.22's ServeMux routes by method and path, and log/slog is structured logging in the
standard library. Fiber, which the evrone template uses, runs on fasthttp, which isn't net/http:
`httptest`, otelhttp and most middleware need adaptors, and oapi-codegen needs a separate
middleware for it.

**Decision.** The server is net/http: the generated router on a ServeMux, middleware as
`func(http.Handler) http.Handler` (`internal/platform/httpx`), `httptest` in tests. Logs are
`*slog.Logger`, JSON on stdout, passed in, never a global.

**Consequences.** What an agent knows of Go's standard library applies as it is. A dependency is
added for what the standard library doesn't do (kin-openapi, oapi-codegen's runtime, caarlos0/env).

**Revisit if.** Profiling shows net/http is the bottleneck, which it rarely is for a JSON API.
