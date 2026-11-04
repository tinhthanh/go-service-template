# Changelog

What a client of the service would notice, newest first.

## Unreleased

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
