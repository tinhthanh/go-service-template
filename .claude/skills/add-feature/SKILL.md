---
name: add-feature
description: Add a feature package (internal/<feature>/) that answers operations of the client's contract from its own table — migration, queries, store, store in memory, service, handler, wiring and tests. Use when asked to build a resource the contract has and no feature serves yet (tasks, files, search…), or "make <resource> work against the server".
---

# Add a feature

`internal/projects/` is the example to copy: a list (`Page`), one record, writes by role, a scope
by workspace. Read it before you write. The compiler, `internal/archtest` and `make check` refuse a
half-wired feature; nothing refuses a missing test, so the tests below are the checklist's end.

1. **Read the operations.** `grep -n "(GET\|(POST\|(PATCH\|(DELETE" internal/api/api.gen.go` lists them by
   path; the generated `<Op>RequestObject` and `<Op>200JSONResponse` are what you take and give. The
   client's mock (`lib/mock.ts` in app-shell-template) is how they behave today: copy its refusals
   (status, code, field, words) and its defaults, and say under Assumptions what you decided beyond it.
2. **Migration** (the `add-migration` skill): `migrations/NNNN_<feature>.up.sql` and `.down.sql`. A
   record of a workspace has `workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE`;
   a name unique in its workspace is a unique index on `(workspace_id, lower(name))`.
3. **Queries**: `internal/<feature>/queries.sql`, and a block for it in `sqlc.yaml` (copy projects',
   change both paths). `SELECT *` and `RETURNING *` give one model; a list is one static query with
   optional filters (`@q::text = '' OR …`), `CASE` per sort column, and a `Count…` with the same
   filters. Words typed are matched with `strpos`, never `LIKE`. Then `make generate`.
4. **Service** (`service.go`): the package comment says what it is and its rules. Its `Store`
   interface is declared here, small. Each method asks who is calling (`auth.Required`; what their
   role may do is the contract's `x-permission`, checked before the method runs, so a new action
   is the client's to add to `x-roles` first), scopes to `p.Belongs(workspace)` (another's is `problem.ErrNotFound`),
   and refuses a value with `problem.Invalid(field, "words for a person")`. Errors wrapped with
   `fmt.Errorf("<feature>: <step>: %w", err)`.
5. **Store** (`store.go`): `PGStore` over the generated queries; `pgx.ErrNoRows` is `problem.ErrNotFound`.
   **Store in memory** (`<feature>test/store.go`): the same interface, filtering and sorting as the SQL does.
6. **Handler** (`handler.go`): `type Handler struct{ svc *Service }`, one method per operation with
   the generated signature, mapping the request into the service's types and its answer into the
   generated response. No rule lives here.
7. **Wiring** (`internal/server`): an alias and an embedded field in `operations` (`server.go`), a
   field in `Features`, the PG store in `Wire` (`wire.go`), and the memory store and handler in
   `servertest.New` (with fixtures a test can name, as `servertest.Vault`).
8. **Tests**:
   - `handler_test.go` (package `<feature>_test`): `servertest.New(t)`, `s.As(servertest.Member)`,
     `s.Do(…)`: one test per behaviour the client sees, named as a sentence (each answer is checked
     against the contract for you); the roles, another workspace's record, each refusal on its field.
   - `store_test.go` (`//go:build integration`): `dbtest.New(t)`, the queries' filters, sort, total,
     uniqueness and defaults on Postgres.
   - A rule no call shows: a table test on the service with the memory store.
9. `make check`, then `make verify` (Docker); a line under `## Unreleased` in `CHANGELOG.md`; an ADR
   if you decided something the next person would ask about. Then the `open-pr` skill.
