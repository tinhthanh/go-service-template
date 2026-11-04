---
name: fix-ci
description: Find out why a pull request's CI is red and fix it, running only what failed — make verify's steps, the evidence and guardrails gates. Use when CI fails, when a PR shows a red check, or when asked why a build is broken.
---

# Fix a red CI

```sh
gh pr checks <n>                                   # verify (make verify) or gates
gh run view <run-id> --log-failed | tail -60       # the step and its last lines
```

- **verify** is `make verify`, step for step; run the one that failed:
  - `gen-check`: `make generate` and commit what it changed (the contract or a `queries.sql` moved).
  - `lint`: `make lint`; the finding names the rule. Fix the code, never `.golangci.yml`.
  - `test-integration`: `go test -race -count=1 -tags integration ./internal/<pkg>/ -run '<Test>'`
    (Docker). A test green here and red there is a race or an order: `-count=10` on it alone.
  - `mod-check`: `go mod tidy` and commit `go.mod`/`go.sum`.
  - `vuln`: `go tool govulncheck ./...` names the module and its fixed version: `go get <module>@<fixed>`,
    or a `toolchain` line in `go.mod` for the standard library.
- **gates** says in one line what the pull request lacks. Fix the text with
  `gh pr edit <n> --title … --body-file …`: the workflow runs again on the edit. A guarded file
  changed: say why under Guardrail changes and `gh pr edit <n> --add-label guardrail-change`.
- A failure that isn't yours: the same command on `main` (`git stash; …; git stash pop`), its last
  lines in the PR, and an issue. Two guesses that don't explain it are the end: note, issue, move on.
- Push the fix as a new commit; never `--no-verify`, never a force push.
