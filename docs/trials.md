# Trials

Each trial gives a fresh agent a product task, told only as a person would write it, in a worktree
of the template (the `trial` skill), and scores what comes back against the repository's rules. What
it got wrong is a finding about the template, fixed here; the trial's own code is not merged.

## Trial 1 — tasks (2026-11-04)

The task: "build tasks on the server: tasks, task, createTask, updateTask and deleteTask answer
501; make them work the way the client expects." Run as a subagent, so the repository's Claude
Code hooks didn't fire: it measured AGENTS.md, the skills, the checks and the gates.

**Score.** `make verify` green (tasks' handler and store tests included); `gates evidence` green on
its own PR.md; `gates guardrails` asked for the label, for `sqlc.yaml`; no rule bent (no edit to the
contract or generated code, its one `//nolint` with a reason, no skipped test). It found and followed
`add-feature`, ran the `reviewer`, proved two of its tests fail without their fix, and ran the server
on Postgres by hand. 30 files, one commit.

**What it found, and what changed.**

| Finding | Change |
|---|---|
| Every new feature edits `sqlc.yaml`, a guarded file, so needs the label | `sqlc.yaml` is no longer guarded: it lists features, not checks |
| `make gates` reads only what is committed, and every commit not on `origin/main` | `open-pr` says commit first, and `BASE_SHA` for a branch off a branch |
| `make db-up && make seed` fails: `seed` needs `DATABASE_URL` | `make run` and `make seed` read `.env`, or `.env.example` |
| AGENTS.md says a contract refusal is 400; the skill says copy the mock's status (422) | AGENTS.md: the client reads `code` and `field`; 400 before the handler is right |
| A refusal of `anyOf` (a day that isn't one) named no field | `invalid` falls back to the value's pointer (the trial's fix, taken) |
| A PATCH's `null` ("take it off") and a field left out are both nil in the generated code | `httpx.Nulls` and `SentNull` (the trial's, taken), and a line in AGENTS.md |
| No rule on a feature reading another's table | AGENTS.md: read it in SQL to show it, never write it |
| `internal/projects`, the example, answered 500 to a cursor past int32 | `offsetOf` parses as int32; a test with three cursors that aren't one |

Open: what "today" is for a count such as tasks' `late` (the server's UTC day, the trial's choice)
has no rule yet.
