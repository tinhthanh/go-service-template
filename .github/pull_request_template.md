## What the client gets

<!-- One or two sentences. The title is a lowercase conventional subject: feat: …, fix: …, docs: … -->

## Proof

- [ ] `make verify` is green (paste its last lines)
- [ ] Every behaviour I changed has a test, named below, that fails without the change
- [ ] A change a client would notice has a line under `## Unreleased` in `CHANGELOG.md`
- [ ] A decision the next person would ask about has an ADR in `docs/decisions/`

```
# make verify
```

**Tests added or changed**

-

<!-- No test possible for a change under internal/ or cmd/? Say why on one line: "no-test: <why, 15 characters or more>". -->

## Guardrail changes

<!-- Only if the checks changed (tools/gates guarded: .golangci.yml, Makefile, .github/, .claude/, AGENTS.md, archtest, contracttest, dbtest, hooks…): what and why, and the guardrail-change label. -->

## Assumptions

<!-- What the task left open and you decided. "None." if nothing. -->

## Not done

<!-- What you left out, and why. "Nothing." if it is all there. -->
