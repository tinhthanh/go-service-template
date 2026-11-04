# 0002: A package per feature, its rules a test

**Status:** accepted, 2026-11-04

**Context.** A layered layout (entity, use case, repo, controller, each a directory; evrone's
go-clean-template) makes one feature touch eight to ten files in six places, and its rule (inner
layers don't import outer ones) was prose. Agents keep a change local when the place to put it is
one directory, and keep a rule when a check fails without it. ardanlabs/service and the Go team's
module-layout guidance keep code by what it is about.

**Decision.** `internal/<feature>/` holds a feature: its handler (the generated methods it builds),
its service (the rules) and its store, with their tests. A feature imports `internal/api` and
`internal/platform`, never another feature: it declares the small interface it needs, and
`internal/server` passes the other feature in. `internal/platform` knows no feature.
`internal/archtest` reads every package's imports (`go list`) and fails with what to do instead.

**Consequences.** A feature is read and changed in one place; two features share only what
`platform` gives them or what `server` wires. A rule added is a case in archtest's table, tested
both ways.

**Revisit if.** Features need to share a model enough that interfaces at every border cost more
than they keep apart.
