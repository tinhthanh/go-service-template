# Changelog

What a client of the service would notice, newest first.

## Unreleased

- A request whose value none of a field's `anyOf` takes (a day that isn't one) names that `field`.
- A projects `cursor` past what a page can start at begins at the top instead of a 500.
- `GET /api/workspaces` is the caller's workspaces; `POST /api/workspaces` makes one they belong to
  at once, its id made from its name (`w-acme-labs`).
- Projects: a page of the caller's that match (`workspace`, `q`, `status`, `tags`, `sort`,
  `cursor`, `limit`) with the total of every match; one; making, changing and deleting one; and
  the tags in use. Another workspace's project is 404; a viewer may only read, only an owner deletes;
  a name taken in the workspace is 422 on `name`.
- Signing in (`POST /api/session`) sets an httpOnly session cookie and a refresh cookie for
  `/api/session/refresh`; `GET /api/session` is who is signed in (401 when nobody is, or the session
  has ended); refreshing gives new cookies and the old ones stop; signing out ends the session.
- A write without `X-Requested-With: XMLHttpRequest` is refused (403).
- A value the rules refuse is 422 `invalid` with its `field`, as the client's mock answers it.
- Every operation of the client's contract is routed under `/api` and answers 501 `unavailable`
  until it is built; a request the contract refuses is 400 `invalid` with the `field` it names.
- Every answer carries `X-Request-ID` and security headers; the client's origins may call with
  cookies (`CORS_ORIGINS`).
- `GET /healthz` answers 204 while the server is up.
