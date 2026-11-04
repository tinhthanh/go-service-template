# 0012: The contract says who may: x-permission is checked before every operation

**Status:** accepted, 2026-11-04

**Context.** What a role may do was kept three times by hand: the client's `ACTIONS`
(`lib/permissions.ts`), its mock's `guard(true, "project.edit")` calls, and this server's
`platform/auth/permissions.go` with an `auth.Allowed` in each service method. Nothing held them
equal, and a feature that forgot its `auth.Allowed` answered a viewer's write.

**Decision.** The client's contract carries the table (its ADR 0012, "Since"): each operation's
`x-permission` (`public`, `signed-in`, or an action) and a top-level `x-roles` (each action and
the roles that may take it); its `check:contract` holds `ACTIONS` and the mock to them. Here,
`auth.RulesOf` reads both from the embedded contract at start, and refuses one where an operation
has no `x-permission` or names an action `x-roles` lacks. A strict middleware checks each call's
operation before its handler, or its 501 stub, runs: 401 for nobody, 403 for a role left out. A
service asks only who is calling (`auth.Required`) and whose the record is (ADR 0011).

**Consequences.** No Go code names a role or an action. A new action, or a role's change, is the
client's edit to its contract, then `make contract-sync`. An operation not built yet already
refuses as it will. A rule that depends on the record (its owner may delete it) is still the
service's, beside the contract's.

**Revisit if.** Permissions depend on the record often enough that the contract should carry them.
