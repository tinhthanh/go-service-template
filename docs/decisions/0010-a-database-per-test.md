# 0010: A real Postgres for stores, a database per test, the rest in memory

**Status:** accepted, 2026-11-04

**Context.** A store's SQL is right only against Postgres: a mock of pgx proves the mock. But a
database in every test makes `make check`, which runs before every stop, slow and dependent on
Docker. The evrone template ran its suite against one docker-compose stack whose data outlived a
run, and its second run failed on names the first had used.

**Decision.** A store's tests carry the build tag `integration` and take `dbtest.New(t)`: one
Postgres container per test binary (testcontainers), the migrations applied once to a template,
and each test a database of its own cloned from it, so they run in parallel and see no one else's
rows. `make test` (and so `make check`) leaves them out; `make test-integration` and `make verify`
(CI) run them. A feature's HTTP tests and rules run on its store in memory (`<feature>test`) under
`servertest`, which uses the real server, middleware and contract checks.

**Consequences.** `make check` needs no Docker and stays seconds; `make verify` needs it. A store in
memory must behave as the real one on what the tests rely on (a session that has ended, a token
that works once), and the store's own tests are where that is proven on Postgres.

**Revisit if.** The in-memory stores drift from Postgres in a way a test relied on: test that path on
Postgres instead.
