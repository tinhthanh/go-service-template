# 0009: Sessions are kept on the server, a cookie holds their token

**Status:** accepted, 2026-11-04

**Context.** The client keeps nobody's token in storage: its session is an httpOnly cookie, a 401
asks `POST /session/refresh` once, and every write carries `X-Requested-With` (its ADR 0011). A JWT
in that cookie would be readable by nothing on the page, but it can't be ended before it expires:
signing out, or a person removed, leaves it working.

**Decision.** A session is a row: the hash (SHA-256) of a random 32-byte token, the hash of a refresh
token, the person, and when each ends. The `session` cookie (Path `/`) carries the token; the
`refresh` cookie, sent only to `<prefix>/session/refresh`, carries the other. Both are httpOnly,
Secure (`COOKIE_SECURE`, off only for plain HTTP on a laptop) and SameSite=Lax. Refreshing swaps
both tokens, so a refresh token works once. Signing out deletes the row. A wrong email and a
wrong password are refused alike, after the same bcrypt work. `platform/auth`'s middleware puts
the person (with their workspaces) on the request; `auth.CSRF` refuses a write without the
client's header.

**Consequences.** Every call reads a row (an index on the hash); a session ends the moment it is
deleted. The role table is in two places, the client's `lib/permissions.ts` and
`platform/auth/permissions.go`, until the contract names each operation's action (phase 4).

**Revisit if.** A call's read of the session row shows in latency: cache it for seconds, never longer
than a sign-out may take to apply.
