# Changelog

What a client of the service would notice, newest first.

## Unreleased

- Every operation of the client's contract is routed under `/api` and answers 501 `unavailable`
  until it is built; a request the contract refuses is 400 `invalid` with the `field` it names.
- Every answer carries `X-Request-ID` and security headers; the client's origins may call with
  cookies (`CORS_ORIGINS`).
- `GET /healthz` answers 204 while the server is up.
