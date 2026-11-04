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
2. **The rule in the service** (`internal/<feature>/service.go`): who may (`auth.Allowed`), whose
   (`p.Belongs`), what is refused (`problem.Invalid(field, words)`, `problem.ErrNotFound`). The
   request's shape is already checked against the contract before your code runs: don't re-check it.
3. **A query** if it needs one: in the feature's `queries.sql`, then `make generate` (the after-edit
   hook already ran sqlc and named a wrong column). A method on the store interface, `PGStore`, and
   the store in memory.
4. **The handler method**: map in, call the service, map out. A write that sets cookies or headers
   the contract doesn't name answers a type of its own with the `Visit<Op>Response` method
   (`session.withCookies`).
5. **Tests**: in `handler_test.go` through `servertest`, each behaviour a test named as what the client
   gets; a store test (`integration`) for a new query. The test of `internal/server` that lists
   operations answering 501 names only ones still not built: move yours out of it.
6. `make check`; a CHANGELOG line; the `open-pr` skill.
