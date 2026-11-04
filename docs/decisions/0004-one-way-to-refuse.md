# 0004: One way to refuse: problem's errors, the contract's Error

**Status:** accepted, 2026-11-04

**Context.** The client reads every refusal as the contract's `Error` (`{error, code, field}`):
words for a person, a code for the app, and for `invalid` the field the form puts the words on.
Each handler writing its own would differ in status, code or words, and leak the raw error.

**Decision.** A feature returns an error: `problem.ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`,
`ErrConflict`, `problem.Invalid(field, words)`, wrapped as it likes. `problem.Write` maps it to
its status and code once, for the strict server's errors, the request validator's and a recovered
panic. Anything else is a 500 with plain words, its error logged with the request. RFC 9457
problem+json was not chosen: the client's contract decides the shape.

**Consequences.** A new kind of refusal is a line in `problem.kinds` and a test. A handler doesn't
build an `Error` itself.

**Revisit if.** The client's contract changes its Error.
