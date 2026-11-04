# Decisions

Why the service is the way it is. One file per decision: its context, what was decided, what
follows, and when to revisit it. A new one takes the next number; one replaced says so and stays.

| # | Decision |
|---|---|
| [0001](0001-the-contract-is-the-clients.md) | The contract is the client's, and the server is generated from it |
| [0002](0002-a-package-per-feature.md) | A package per feature, its rules a test |
| [0003](0003-the-standard-library-first.md) | net/http and log/slog, not a framework |
| [0004](0004-one-way-to-refuse.md) | One way to refuse: problem's errors, the contract's Error |
| [0005](0005-lint-a-tool-can-decide.md) | Lint what a tool can decide; drop what only moves blank lines |
| [0006](0006-ci-is-make-verify.md) | CI is make verify |
| [0007](0007-hooks-for-agents.md) | Checks at the edit and at the stop, for agents |
| [0008](0008-sqlc-for-stores.md) | Stores in SQL with sqlc, on pgx |
| [0009](0009-sessions-kept-on-the-server.md) | Sessions are kept on the server, a cookie holds their token |
| [0010](0010-a-database-per-test.md) | A real Postgres for stores, a database per test, the rest in memory |
| [0011](0011-what-isnt-yours-isnt-there.md) | What isn't yours isn't there: another workspace's records are 404 |
| [0012](0012-the-contract-says-who-may.md) | The contract says who may: x-permission is checked before every operation |
| [0013](0013-a-day-is-the-servers-in-utc.md) | A day with no time is compared in UTC |
