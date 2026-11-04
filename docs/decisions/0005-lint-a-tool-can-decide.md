# 0005: Lint what a tool can decide; drop what only moves blank lines

**Status:** accepted, 2026-11-04

**Context.** Agents follow a check more reliably than a paragraph. The Google and Uber Go style
guides, Effective Go and Code Review Comments agree on most of what matters in a service (wrapped
errors, context first, no globals, small interfaces, return structs, named errors), and
golangci-lint v2 has a linter for most of it. evrone's set also has wsl and nlreturn, which only
move blank lines about and cost an agent a round trip on most edits, and mnd, which turns every
number into a constant.

**Decision.** `.golangci.yml` enables the linters that decide a rule from those guides (errorlint,
wrapcheck, errname, err113, nilnil, nilerr, containedctx, contextcheck, ireturn, iface,
recvcheck, gochecknoglobals, testifylint, modernize, revive's naming and comment rules, gosec,
among others), with gofumpt and goimports as formatters; drops wsl, nlreturn and mnd; and requires
a `//nolint` to name its linter and say why. Errors from this module's own packages need no
wrapping (wrapcheck's ignore list); another module's always do.

**Consequences.** A finding names the rule; the after-edit hook shows it at once. A rule the
guides keep that no linter decides (handle an error once) is in AGENTS.md and the reviewer's eye.

**Revisit if.** A linter proves to be mostly noise for agents: drop it here, with the reason.
