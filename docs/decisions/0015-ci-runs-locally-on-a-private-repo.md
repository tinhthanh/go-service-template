# 0015: Where Actions minutes cost, CI runs on a laptop, reviewed, and says so

**Status:** accepted, 2026-11-05

**Context.** A service made from this template is often private, and on GitHub Free a private
repository pays for Actions past a small allowance and gets no rulesets: nothing stops a red pull
request merging. `make verify` is most of CI's minutes (the stores on Postgres in Docker,
govulncheck). The client, app-shell-template, settled the same question in its ADR 0017.

**Decision.** The repository variable `CI_MODE=local` (`gh variable set CI_MODE --body local`)
skips the verify job. `make proof` (`tools/proof`) is its stand-in: `make verify` on a clean copy
of HEAD, the gates on `PR_TITLE`, `PR_BODY` and `PR_LABELS`, then Claude Code's reviewer agent on
the diff, answering with a verdict. The review is read-only by more than its word: only Read,
Grep, Glob and Bash are loaded, no settings (the project's allowlist lets `git commit` and `make`
through) and no MCP server, in `dontAsk` mode with only `git diff/log/show` and `gh pr view/diff`
allowed and git's `--output` denied. It writes `evidence/review.md` and `evidence/verify.json`
(the commit, a hash of every tracked file but `evidence/`, each step, the verdict). `make
proof-check` fails unless the record is this tree's, every step green and the verdict "ready"; the
pre-push hook runs it on each branch pushed where `CI_MODE` is `local`, and so does the gates job
on the pull request's head. The verify job still runs when dispatched and on Dependabot's pull
requests. `evidence/` is committed only there; this template, public, keeps the whole of CI.

**Consequences.** A pull request costs one short job, and every one is reviewed. The record is what
a laptop said, not a signature (an agent could write one): its tree must be the head's, and the
owner can dispatch `ci.yml` on any commit. A review costs Claude tokens; its verdict is advice, and
the owner still reads before merging. Without rulesets the hook is the gate, and a hook can be
skipped, and lets a push by when `gh` can't ask (the gates job still checks). Read-only rests on
Claude Code's own refusals (a redirect, a chained command) and on git config running no diff
driver. What `main` gets from a squash merge is a tree nobody ran, and two open pull requests each
commit `evidence/`: the second rebases, and proves again.

**Revisit if.** The plan changes (paid minutes, rulesets) or a self-hosted runner is set up (its
minutes are free): then `CI_MODE` goes, and `make proof` stays as the run before a push.
