# Working on this service

A Go backend for a browser client built from `app-shell-template`. The client owns the API: its
`api/openapi.json` is copied here and generated into Go, and this service answers it. Go 1.27,
net/http, log/slog, oapi-codegen (strict server), kin-openapi, Postgres through pgx and sqlc,
golang-migrate. Why each choice: `docs/decisions/`.

## Commands

Everything goes through `make` (`make help`); the hooks and CI run the same targets.

```sh
make check        # the inner loop: generated code current, lint, unit tests (-race); no database
make verify       # what CI runs (it runs exactly this): check, the stores on Postgres (Docker), tidy, govulncheck
make generate     # internal/api from api/openapi.json, and each feature's db/ from its queries.sql (sqlc)
make contract-sync FROM=../app-shell-template   # copy the client's contract in, then generate
make fmt          # gofumpt + goimports
make db-up && make seed   # a Postgres on :5432, and the acme people (owner, member, viewer @acme.test)
make run          # the server on :8080, settings from the environment (.env.example)
make setup        # once: git hooks (format and lint on commit, make check on push)
```

Claude Code runs a hook after each edit (format the file, `go vet` and lint its package; after a
`.sql` file, sqlc against the migrations, so a wrong column shows at once) and `make check` before it
stops (`.claude/settings.json`, `tools/hooks`): fix what they say before the next edit. Generated
files and the contract are refused. Force pushes, pushes to `main` and `--no-verify` are refused.

## Map

```
api/openapi.json        the client's contract: never edited here (make contract-sync)
internal/api/           generated from it: models, router, StrictServerInterface, Unimplemented (501)
internal/server/        puts it together: features' handlers, contract validation, middleware
internal/platform/      what every feature uses and no feature owns: config, httpx, problem, auth, db,
                        logging, and for tests contracttest and dbtest
internal/<feature>/     one package per feature: handler.go, service.go, store.go, queries.sql, db/ (sqlc),
                        <feature>test/ (its store in memory), and their tests. projects is the example
                        of a list (filters, sort, a Page); session of cookies; workspaces of a transaction
internal/server/servertest/  the real server on stores in memory, for a feature's HTTP tests
migrations/             the schema, golang-migrate pairs (NNNN_name.up.sql/.down.sql), embedded
internal/archtest/      the architecture's rules, as a test
cmd/server/             main
tools/                  stubgen (the 501 stubs), hooks (Claude Code's)
```

## The contract

- `api/openapi.json` is the client's. A change to it is made in the client, then synced. The
  after-edit hook refuses an edit to it, and to any `*.gen.go`.
- Every operation is routed. One no feature has built is answered by `api.Unimplemented`: 501,
  code `unavailable`. Building it is a method on a feature's handler with the generated
  signature; `internal/server` embeds the handler, and its method takes the stub's place.
- Every request is held to the contract before it reaches a handler (kin-openapi). A refused one
  is 400 `invalid` with the `field` it names, so a handler never re-checks the shape.
- A handler returns the generated response types, or an error from `internal/platform/problem`:
  `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrConflict`, `problem.Invalid(field, words)`.
  `problem.Write` turns it into the contract's `Error` (`{error, code, field}`) once, for every
  operation. Anything else is a logged 500; the raw error never reaches the client.

## Who is calling

- The session is a cookie (httpOnly, Secure, SameSite=Lax) holding a random token; the database
  keeps only its hash, so sign-out ends it. `platform/auth` puts the caller on the context.
- What an operation needs is the contract's: its `x-permission` (`public`, `signed-in`, or an action)
  and `x-roles` (each action's roles), checked before any handler or stub runs (`auth.Rules`), so a
  feature can't forget it and nothing in Go names a role. A new action is the client's change
  (contract and its `lib/permissions.ts`), then `make contract-sync`. A feature's service asks only
  who is calling (`auth.Required(ctx)`) and whose the record is: one of a workspace the caller isn't
  in is `problem.ErrNotFound`, as if it weren't there (`p.Belongs(workspace)`).
- Every write carries `X-Requested-With: XMLHttpRequest` (CSRF); the client sends it.
- A refusal of a value the rules don't allow is `problem.Invalid(field, words)` (422, as the client's
  mock answers); a request the contract refuses is 400 before the handler runs. The client reads
  `code` and `field`, not the status, so where the mock answers 422 for a shape the contract
  already refuses (a day that isn't one), 400 is right.
- A PATCH field the contract lets be `null` ("takes it off") is generated as a pointer, nil for null
  and for left out alike: `httpx.SentNull(ctx, "due")` tells them apart.

## Lists

A list the client reads a page at a time answers the contract's `Page`: `items`, `total` (every
match, worked out by the store, never the page's length), and `next` while more are left. The
cursor is the offset as a string, as the client's `lib/pages.ts` writes it; one that isn't starts at
the top. A page is 50 unless asked; more than 200 is 422 on `limit`. Filters and sort are one static
query in `queries.sql` (optional filters, `CASE` per sort column), so sqlc checks all of it; words a
person typed are matched with `strpos`, never `LIKE`, so they are never a pattern.

## Rules

- **A package per feature.** A feature imports `internal/api` and `internal/platform`, never another
  feature: it declares the small interface it needs, and `internal/server` passes the other in.
  `internal/platform` knows no feature. `internal/archtest` fails the build otherwise. In SQL a
  feature may read another's table to show it (a task's project name, by JOIN) but never writes it.
- **Errors** are wrapped with `%w` and context (`fmt.Errorf("projects: store: %w", err)`), compared
  with `errors.Is`/`errors.As`, handled once (returned or logged, never both).
- **Context** is the first argument of anything that does I/O, and is never kept in a struct.
- **Constructors return structs**; interfaces are declared where they are used, small.
- **No globals** but sentinel errors and compiled regexps; **no `init()`**.
- **Logging** is the `*slog.Logger` passed in, never `fmt.Print` or `log`; never a person's data or
  what they typed (the request log has no query and no body).
- **A `//nolint`** names its linter and says why (`nolintlint`). Don't loosen `.golangci.yml` to
  get a change through.
- **Fixtures are synthetic** ("acme"); no secrets in code or logs; settings are env vars
  (`internal/platform/config`, a line in `.env.example` each).

## Tests

| You changed | The proof |
|---|---|
| A handler | `contracttest.Client` calls it through the real server; every answer is checked against the operation's response schema |
| A handler, or a rule a caller sees | the feature's `handler_test.go` (package `<feature>_test`): `servertest.New(t)`, `s.SignIn(email)`, `s.Do(…)`; every answer is checked against the contract |
| A service's rule no call shows | a table test beside it, against `<feature>test`'s store in memory |
| A store | `store_test.go`, build tag `integration`: `dbtest.New(t)` is a database of its own on a real Postgres (Docker), run by `make verify` |
| Middleware, problem, config | a test beside it |
| The architecture | `internal/archtest` (add a case to its table when a rule changes) |

Tests are named as what the caller gets (`TestServer_ARequestTheContractRefusesIsInvalidOnItsField`),
use testify's `require` for what must hold before going on and `assert` for the rest, and never
wait on time: `s.Clock().Advance(d)` moves the server's clock (sessions take `now` as a function).

## Recipes

Each change has a skill in `.claude/skills/` that is its checklist: load it before you start, and
follow it step by step. `add-feature` (a resource no feature serves), `add-operation` (one more
operation of a feature), `add-migration`, `open-pr` (CI's gates, run here first), `fix-ci`, and
`trial` (measure the template on a fresh agent). The `reviewer` agent
(`.claude/agents/reviewer.md`) reads a diff against these rules before a pull request.

## Done means

1. `make check` is green; before a pull request, `make verify` (Docker running).
2. Every behaviour changed has a test that fails without the change.
3. A choice the next person would ask about has an ADR in `docs/decisions/`.
4. A change a client would notice has a line under `## Unreleased` in `CHANGELOG.md`.
5. The work is on a branch, as a pull request against `main`, with a lowercase conventional subject
   (`feat: …`, `fix: …`, `docs: …`, `chore: …`) that says what the client gets, and its body from
   the template. CI's gates (`make gates`) refuse a behaviour change with no test changed (or a
   `no-test: <why>` line), a client-visible change with no CHANGELOG line, a `feat:` without
   Assumptions and Not done, an added `t.Skip` or bare `//nolint`, and a change to the checks
   themselves without the `guardrail-change` label.
