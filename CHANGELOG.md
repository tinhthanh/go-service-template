# Changelog

What a client of the service would notice, newest first.

## Unreleased

- A JSON body with text that isn't UTF-8 (a lone surrogate included) or a name sent twice in one
  object is refused with 400 `invalid` on the field it is in, where it was kept with U+FFFD in
  place of what was sent, or the last of the two kept (ADR 0016).
- `POST /api/files` keeps the file the client sends (its name from `X-File-Name`, up to 5 MB;
  more is 422 on `attachment`), and a task names it by the id answered: its `attachment` shows it,
  `null` takes it off, and a file someone else sent is 422 on `attachment`. Every operation of the
  client's contract is now built.
- A request's body past 8 MB is refused with 413 `too_large`.
- `GET /api/search?q=` finds the caller's projects and tasks, each kind ranked on its own and at most
  five of each, as the client's palette ranks them: the name itself, one that starts with the
  letters, one with a word that does, one that has them, then a match in the rest. A project's
  detail is its owner, a task's its project's name.
- Tasks: a page of the caller's that match (`workspace`, `q` in the title or the project's name,
  `status`, `project`, `hideDone`, `sort` by `title`, `due` or `updated`, `cursor`, `limit`) with the
  total of every match and `summary.late` (past their day, in UTC, and not done); one, with its
  project's name as it is now; making one (to do, in the caller's first workspace unless named);
  changing one, where `null` takes the due day or the project off; and deleting one. Another
  workspace's task is 404; a viewer may only read, only an owner deletes; no title is 422 on
  `title`, a project not of the task's workspace 422 on `project`, and any file 422 on `attachment`
  (files can't be sent yet).
- A request with a field the contract doesn't have, or without one it needs, names that `field`
  for a body with an `anyOf` in it too (a task's).
- What each operation needs is read from the contract's `x-permission` and `x-roles`, and checked
  before any handler: an operation not built yet refuses a caller its role doesn't allow (401, 403)
  before it answers 501.
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
