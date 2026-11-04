---
name: open-pr
description: Write and open a pull request that passes CI and its gates the first time — make verify, the title, the body from the template with the tests named, Assumptions and Not done, the changelog line, the guardrail-change label. Use when work is finished and is to be pushed, when asked to open or write a PR, or to write PR.md in a trial.
---

# Open a pull request

CI runs `make verify` and two gates that read the pull request (`tools/gates`): `evidence` and
`guardrails`. All three run here first, so nothing is learnt from a red CI.

1. **`make verify`** green (Docker running), its last lines kept to paste.
2. **The title**: a lowercase conventional subject that says what the client gets, not what the
   code does: `feat: tasks can be filtered by project`, `fix: a refused save names its field`.
   `chore:`, `docs:`, `test:`, `ci:`, `refactor:`, `build:` say a client won't notice; a change
   under `internal/` a client would notice is `feat:` or `fix:`.
3. **The body**: `.github/pull_request_template.md`, every section in plain words. The tests by
   name under their files; `no-test: <why, 15 characters or more>` on a line of its own only when a
   change under `internal/` or `cmd/` has none possible. Assumptions: what the task left open and
   you decided (a `feat:` without it is refused). Not done: what you left out, and why.
4. **CHANGELOG.md**: a line under `## Unreleased` for a change a client would notice.
5. **Guardrails**: a change to the checks (the list in `tools/gates/gates.go`, `guarded`) says what
   and why under Guardrail changes and carries the `guardrail-change` label. Never loosen a check
   to get a change through.
6. **The gates, as CI will run them**, on what is committed (commit first: they diff
   `origin/main...HEAD`, as CI diffs the pull request's base):
   ```sh
   PR_TITLE="feat: …" PR_BODY="$(cat pr.md)" PR_LABELS="" make gates
   ```
   On a branch made from another branch, `BASE_SHA=<that branch>` reads only your commits.
7. **Open it**, on a branch, never `main`:
   ```sh
   git push -u origin <branch>                  # pre-push runs make check
   gh pr create --base main --title "…" --body-file pr.md [--label guardrail-change]
   ```
   In a trial, or wherever you may not push: write the title and body into `PR.md` and commit it.
8. Watch it: `gh pr checks <n>`. Red: the `fix-ci` skill.
