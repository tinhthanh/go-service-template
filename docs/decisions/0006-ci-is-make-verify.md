# 0006: CI is make verify

**Status:** accepted, 2026-11-04

**Context.** In the template this one pairs with, a build that passed on a laptop failed on CI, and
the fix was a local command that reads CI's steps (`pnpm ci:local`). A list of steps kept in two
places drifts; evrone's `make pre-commit` and its CI already differed.

**Decision.** CI's one job runs `make verify`, nothing else: `make check` (generated code current,
lint, every test with the race detector), `go mod tidy -diff`, govulncheck. A step added is added
to the Makefile, so it runs in both. The pre-push hook runs `make check`.

**Consequences.** `make verify` green here is CI green there, but for what the runner's machine
differs in (a race that shows only on a slower one). Steps that need Docker (a store's Postgres)
join `make verify` when the first store lands, and run in both again.

**Since (2026-11-04).** A pull request also meets two gates that read its text, a job of its own
since a laptop has no pull request: `evidence` (a behaviour change has a test changed or a
`no-test: <why>` line, a change a client notices a CHANGELOG line, a lowercase conventional title,
a `feat:` its Assumptions and Not done) and `guardrails` (no `t.Skip` or bare `//nolint` added; the
checks themselves, their configs, AGENTS.md and `.claude/` changed only with the `guardrail-change`
label, which CODEOWNERS routes to the owner). `make gates` runs them with the text in env. The
workflow runs on `edited` too, so a body fixed is read again.

**Revisit if.** CI needs a step a laptop can't run (a deploy, a paid scanner): it goes in a job of
its own, said so here.
