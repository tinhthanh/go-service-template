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
has no rule yet. (Settled after trial 2: ADR 0013.)

## Trial 2 — tasks, in a Claude Code session of its own (2026-11-04)

The same task, on main after trial 1's fixes and phase 4 (permissions from the contract), run as a
real Claude Code session in a berth worktree, so the repository's hooks fired.

**Score.** `make verify` green; both gates green with no label (trial 1's `sqlc.yaml` finding
didn't recur); no rule bent (two `//nolint`, each with its reason; no edit to the contract or
generated code). The after-edit hook ran about 75 times and caught two `go vet` errors at the edit
that made them (a function that doesn't exist, a generated type used as another). It found
`add-feature`, ran the `reviewer` and fixed what it said. 25 files, three commits.

**What it found, and what changed.**

| Finding | Change |
|---|---|
| A body with an `anyOf` is refused in kin-openapi's own words, so an extra field had no `field` and a missing one the wrong words | `fieldInReason` reads those too (the trial's fix, taken), and a test on `POST /tasks` |
| `id::text = @id::text`, copied from projects, keeps the primary key's index from serving a read by id | Projects compares UUIDs and answers an id that isn't one without a query; a store test EXPLAINs the store's own query, red with the old one |
| AGENTS.md allows reading another feature's table "to show it", not to check it | Reading to check is allowed too; never writing |
| `archtest` holds a feature's tests too, so another feature's fixtures can't be imported; nothing said so | `add-feature`: make them through the server, or with SQL in a store test |
| `server_test.go`'s list of operations answering 501 names tasks, so building tasks turns it red | `add-feature` says to take yours out |
| "The client reads code and field, not the status" reads as "a test may accept either" | AGENTS.md: a test still pins the status |
| A migration changes every feature's `db/models.go` | `add-migration` says it is expected |
| "Today" for `late` had no rule | ADR 0013: a day is compared in UTC |

Not the template's: the prompt said "commit on your branch" while AGENTS.md says to branch, which
the trial noticed. Its tasks code stays on `trial-2-tasks`, not merged.

## Trial 3 — comments on tasks, client and server together (2026-11-05)

A feature neither repository had, asked of one Claude Code session in two berth worktrees (this
one and `app-shell-template`'s): the client's side first (its contract, mock, view and tests),
then `make contract-sync` and the server's.

**Score.** Both sides green when checked again here: `make verify` (on Docker), `make gates` with
PR.md's title and body and no label; the client's `pnpm check` and both its gates. The contract
here is the client's byte for byte, synced, not edited. `internal/comments` follows `add-feature`
(queries in SQL, a store in memory, servertest, four store tests on Postgres, an ADR, a CHANGELOG
line, no role named in Go); a task of another workspace is 404 to read and to write, and a task
deleted between the check and the insert is 404, not 500. It ran the `reviewer` on both diffs.

**What it found, and what changed.**

| Finding | Change |
|---|---|
| It wrote every Go file from the shell (heredocs, `python3 -`, `sed -i`: 27 writes, 2 by Edit or Write), so the after-edit hook never saw its Go: nothing formatted, vetted or linted at the write, and an edit to generated code or the contract would not have been refused | `before-bash` and `after-bash`: after a shell command, the same checks on every file it changed, each package once; generated code and the contract are refused unless `make` (or sqlc, stubgen) rewrote them |
| A new worktree isn't a trusted workspace, so the allowlist in `.claude/settings.json` is ignored and the session sat waiting on its first command | `trial` says to trust it before the agent starts, and not on the person's behalf |
| PR.md said `make verify` was green; checked again here it was red, because Docker had stopped since | Not the trial's fault, but the score is now taken from a run here, not from PR.md (`trial`) |

Not the template's: the client found a test of its own that fails one run in three on `main` too
(`e2e/tasks.spec.ts:78`); it is the client's to fix. The trial's code stays on `trial-3-comments`.

## Trial 4 — watching tasks, client and server together (2026-11-05)

The second feature across both repositories in one session, after trial 3's fixes (the hooks see
what a shell command writes): watch a task or stop, see who watches it, narrow the list to the
ones you watch; a viewer watches too. The client's contract first, then `make contract-sync`.

**Score.** Both sides green when checked again here: `make verify`, `make gates` (once `PR.md`'s
title line lost its `#`), the client's `pnpm check` and both its gates; the contract is the
client's byte for byte. The shell hook did what trial 3 lacked: of 31 writes, 29 from the shell,
it caught four errors at the write (two `go vet`: the store in memory no longer met the interface,
a type in a test; a lint rule twice). The `reviewer` found three things and the agent fixed them.

**What it found, and what changed.**

| Finding | Change |
|---|---|
| A watch on a task deleted meanwhile was 500 from Postgres (a foreign key) and 404 from the store in memory; trial 3 wrote the same check by hand | `db.Missing(err)` (23503, a constraint named or not), a rule in AGENTS.md, and `add-operation` says both stores answer alike |
| The test of another workspace's task checked the 404, not that nothing was written | `add-operation`: read it back as its owner |
| A field added to `Task` turned red every `assert.JSONEq` on a task | `add-operation` says so, and that it is the point |
| `PR.md`'s first line was `# feat: …`, which the gates take as the title | `open-pr`: the title as GitHub shows it, no `#`; `make proof` strips one |
| `open-pr` says to branch; the trial's prompt said to commit on the branch it had | `open-pr`: in a trial, the worktree's branch |
| The session was this repository's, so the client's hooks never ran on the client's side | AGENTS.md (both): run the other's `check-file`, or open a session there |

