# 0011: What isn't yours isn't there: another workspace's records are 404

**Status:** accepted, 2026-11-04

**Context.** A person belongs to some workspaces. The client lists, opens and changes records of
the one on show, and its mock answers a record it doesn't know with 404 `not_found`. A 403 for a
record of another workspace would tell its id exists, and the client would draw Forbidden where
nothing should be.

**Decision.** A read with no workspace covers every one of the caller's (`auth.Principal.Workspaces`,
read with the session). A workspace asked that isn't theirs, and a record of one, is
`problem.ErrNotFound`, for reading, changing, deleting and making a record in it alike. What a role
may do (`auth.Allowed`) is asked first, so a viewer's write is 403 whatever it names. A project's
name is its workspace's own, in any case (a unique index on `(workspace_id, lower(name))`).

**Consequences.** A feature's service checks `p.Belongs(record.Workspace)` after reading the record,
and scopes every list to the caller's workspaces in the query. A membership added (a workspace
made) applies on the caller's next call, when the session is read again.

**Revisit if.** Records are shared across workspaces, or a role sees every workspace.
