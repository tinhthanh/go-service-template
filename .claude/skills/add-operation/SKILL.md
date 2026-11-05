---
name: add-operation
description: Build one more operation of the client's contract in a feature that already exists (it answers 501 until then) — the service method, the handler method with the generated signature, the query if it needs one, and its tests. Use when asked to implement an endpoint or an operationId of a resource that already has a feature, or when the contract gained one.
---

# Build an operation

Every operation of `api/openapi.json` is routed; one no feature builds is answered by
`api.Unimplemented` (501 `unavailable`). Building it is a method with the generated signature on
the feature's `Handler`: `internal/server` embeds the handler above the stubs, so the method takes
the stub's place. Two features defining the same method fail to compile.

1. **Find it.** `grep -n "<OperationId>(ctx" internal/api/api.gen.go` is the signature;
   `<OperationId>RequestObject` has its path parameters, `Params` (query) and `Body`; its answers are
   `<OperationId>200JSONResponse` (or `204Response`). If the contract changed, `make contract-sync
   FROM=<client>` first, and the hook refuses an edit to `api/openapi.json` or `*.gen.go`.
2. **The rule in the service** (`internal/<feature>/service.go`): who may is the contract's
   `x-permission`, already checked before your method runs (a viewer gets 403 even from the stub);
   the service asks who is calling (`auth.Required`), whose
   (`p.Belongs`), what is refused (`problem.Invalid(field, words)`, `problem.ErrNotFound`). The
   request's shape is already checked against the contract before your code runs: don't re-check it.
3. **A query** if it needs one: in the feature's `queries.sql`, then `make generate` (the after-edit
   hook already ran sqlc and named a wrong column). A method on the store interface, `PGStore`, and
   the store in memory, which answer alike: a write that names a row deleted since the service
   checked it (a task gone while it was being watched) is `problem.ErrNotFound` in both, through
   `db.Missing(err)` in `PGStore`, never a 500.
4. **The handler method**: map in, call the service, map out. A write that sets cookies or headers
   the contract doesn't name answers a type of its own with the `Visit<Op>Response` method
   (`session.withCookies`).
5. **Tests**: in `handler_test.go` through `servertest`, each behaviour a test named as what the client
   gets; a store test (`integration`) for a new query. A refusal of another workspace's record
   proves nothing was written (read it back as its owner), not only the 404; one of nobody signed
   in covers every method of the operation. A field the contract added to a record the feature
   already answers (a task's `watchers`) turns red every test that compares the whole record
   (`assert.JSONEq`): they gain the field, which is the point. Every stub's 501 is held by one test
   of `internal/api` that names no operation, so building one changes no test there.
6. `make check`; a CHANGELOG line; the `open-pr` skill.
