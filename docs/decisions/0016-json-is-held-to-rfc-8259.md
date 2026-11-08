# 0016: A JSON body is held to RFC 8259 as it was sent

**Status:** accepted, 2026-11-08

**Context.** Go 1.25 brought `encoding/json/v2` and `encoding/json/jsontext` as an experiment, and
Go 1.27 (this module's) has them in the standard library, with `encoding/json` itself running on
v2's engine, keeping v1's behaviour. That behaviour is lenient where the contract can't see it:
text that isn't UTF-8 (`"A\xff\xfeB"`, or a lone surrogate `"\ud800"`, which `JSON.stringify`
escapes) is taken and kept with U+FFFD in its place, so a project's name was stored as `A��B` with
a 200; a name sent twice in one object keeps the last. kin-openapi checks the body after
`encoding/json` has decoded it, so it never sees either. The generated server (oapi-codegen) and
kin-openapi call v1's API, which no option of ours changes.

**Decision.** `httpx.WellFormed`, between finding the caller and `httpx.Nulls`, reads a JSON body
(by its Content-Type) with `jsontext`, which holds it to RFC 8259 as v2 does by default: valid
UTF-8, no name twice in an object, one value and nothing after. A body that breaks one is 400
`invalid`, on the top-level field it broke in (from the error's JSON pointer) when there is one.
The rest of the service stays on `encoding/json`'s v1 API: the generated code uses it, and an
answer the service writes is the same in both (empty lists are `[]` by `emit_empty_slices` and the
stores, not by v2's default).

**Consequences.** A client that sends broken text learns it, on the field, rather than finding its
words changed. A body is read once more before the contract reads it, under `MaxBody`'s 8 MB. The
client's mock (app-shell-template) takes such a body, which a browser's `JSON.stringify` only sends
for a string with a lone surrogate in it.

**Revisit if.** oapi-codegen generates for `encoding/json/v2`, or kin-openapi decodes with it: the
check moves into the decoding and this middleware goes. If `GOEXPERIMENT=nojsonv2` is ever set,
nothing here changes: `jsontext` doesn't depend on it.
