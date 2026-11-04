# 0007: Checks at the edit and at the stop, for agents

**Status:** accepted, 2026-11-04

**Context.** Claude Code's guidance is that a rule with no exception belongs in a hook, not in
CLAUDE.md, and that a finding is cheapest while the edit is fresh. The client's template does both
(a check per edit, `pnpm check` at the stop) and refuses force pushes and `--no-verify`.

**Decision.** `.claude/settings.json` runs `tools/hooks`: after an Edit or Write, the file is
formatted and its package vetted and linted, and an edit to `api/openapi.json` or a `*.gen.go` is
refused with what to do instead; before the agent stops with changes, `make check` runs, and red
blocks the stop once. The hook is a Go program, since Go is all the repository needs, built to
`.claude/bin/hooks` before each run: `go run` would turn the exit code 2 that hands a finding back
into a 1. Permissions allow the make targets and read-only git and gh, and deny force pushes,
pushes to main, `--no-verify`, killing processes and reading `.env`.

**Consequences.** An agent hears of a broken rule at the edit that broke it. The first hook run
builds the binary (seconds); later ones are quick.

**Revisit if.** A hook takes long enough to slow every edit: move its slow part to the Stop hook.
