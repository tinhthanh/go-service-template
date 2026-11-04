# 0001: The contract is the client's, and the server is generated from it

**Status:** accepted, 2026-11-04

**Context.** The client (`app-shell-template` and the apps made from it) keeps its API in
`api/openapi.json` and holds its own client and mock to it (its ADR 0016). A server written by hand
beside it drifts: a path renamed, a field added, an answer of another shape, found the day the two
meet. A research round (2026-11) found oapi-codegen v2.8's strict server builds from that very file
(OpenAPI 3.1, checked on two real contracts), and kin-openapi validates requests against it.

**Decision.** The file is copied in, never edited here (`make contract-sync FROM=…`), and
`make generate` turns it into `internal/api`: the models, a net/http router and
`StrictServerInterface`, one method per operation, plus `Unimplemented` (tools/stubgen), which
answers each with a 501 `unavailable`. `internal/server` embeds the features' handlers above the
stubs. Every request is validated against the contract before a handler runs, and `contracttest`
checks every answer a test reads against the operation's response schema.

**Consequences.** An operation the contract gains answers 501 until a feature builds it; one it
loses fails to compile where it was built. `make gen-check` (in `make check`) fails when the
generated code isn't the file's. A handler never re-checks a request's shape. oapi-codegen's 3.1
support is called initial: a construct it can't read is a change to raise with the client, or an
overlay (`api/oapi-codegen.yaml`), never an edit to the file.

**Revisit if.** The contract needs what oapi-codegen can't generate, or the client stops owning it.
