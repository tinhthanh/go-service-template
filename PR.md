feat: tasks are listed, read, made, changed and deleted on the server, with how many are late

## What the client gets

`tasks`, `task`, `createTask`, `updateTask` and `deleteTask` answer as the client's mock does instead
of 501: a page of the caller's tasks with filters, sort, the total and `summary.late`; one task with
its project's name; making, changing (`null` takes a due day or a project off) and deleting one.
A body with an `anyOf` in it (a task's) that is missing a field or sends one the contract doesn't
have now names that `field`.

## Proof

- [x] `make verify` is green (paste its last lines)
- [x] Every behaviour I changed has a test, named below, that fails without the change
- [x] A change a client would notice has a line under `## Unreleased` in `CHANGELOG.md`
- [x] A decision the next person would ask about has an ADR in `docs/decisions/`

```
# make verify
ok  	github.com/tinhthanh/go-service-template/internal/tasks
ok  	github.com/tinhthanh/go-service-template/internal/server
...
No vulnerabilities found.

Your code is affected by 0 vulnerabilities.
```

**Tests added or changed**

- `internal/tasks/handler_test.go`
  - `TestTasks_AreTheCallersWorkspacesNewestFirstWithHowManyAreLate`
  - `TestTasks_LateIsCountedOverEveryMatchOnTheServersDay`
  - `TestTasks_FilterSortAndPageAsTheClientAsks`
  - `TestTasks_ACursorThatIsntOneStartsAtTheTop`
  - `TestTasks_APageOfMoreThanTheMostIsRefused`
  - `TestTasks_AnotherTeamsWorkspaceAndTaskAreNotThere`
  - `TestTask_IsOneOfTheCallersWithItsProjectsName`
  - `TestTask_ShowsItsProjectAsItIsNow`
  - `TestCreateTask_IsToDoInTheMakersFirstWorkspace`
  - `TestCreateTask_TakesAStatusADayAndAProjectOfItsWorkspace`
  - `TestCreateTask_WhatTheRulesRefuseIsInvalidOnItsField`
  - `TestUpdateTask_ChangesWhatItSendsAndKeepsTheRest`
  - `TestUpdateTask_LeavesTheTaskInItsWorkspace`
  - `TestUpdateTask_NullTakesADayAProjectOrAFileOff`
  - `TestUpdateTask_WhatTheRulesRefuseIsInvalidOnItsField`
  - `TestTasks_EachRoleMayDoWhatTheClientsTableSays`
- `internal/tasks/service_test.go`: `TestService_LateIsCountedFromTheDayInUTC`
- `internal/tasks/store_test.go` (integration, Postgres):
  - `TestPGStore_ListFiltersAndCountsTheLateAsTheQueriesSay`
  - `TestPGStore_ListSortsAndPagesWithTheCountsOfEveryMatch`
  - `TestPGStore_ATaskIsReadWithItsProjectsName`
  - `TestPGStore_AProjectIsNamedOnlyInItsOwnWorkspace`
  - `TestPGStore_UpdateSavesTheTaskAsGiven`
  - `TestPGStore_ATaskOutlivesItsProjectButNotItsWorkspace`
  - `TestPGStore_TheDatabaseRefusesWhatTheServiceWould`
  - `TestPGStore_DeleteRemovesTheTask`
- `internal/server/server_test.go`
  - `TestServer_ARequestTheContractRefusesIsInvalidOnItsField`: a task body missing `title`
    ("This is needed.", `field: title`) and one with an unknown `owner` (`field: owner`).
  - `TestServer_AnOperationNotBuiltSaysSoInTheContractsWords`: now `GET /search` and `POST /files`,
    since tasks are built.

## Guardrail changes

None.

## Assumptions

- **Today is the server's day in UTC** for `summary.late` (ADR 0013): the contract sends no zone,
  and the mock uses the browser's day. A task due today is not late; a done task is never late.
- **The order with no `sort`** is the last changed first, then the id, as projects do. The mock
  keeps the newest made first; the two agree until a task is changed.
- **`sort=due:asc` puts tasks with no day last, `due:desc` first**, as the mock's reverse does.
  `title` sorts in any case (`lower(title)`); an unknown column leaves the default order.
- **A task's project** must be one of the task's own workspace, else 422 on `project` ("Pick a
  project in this workspace."), as the mock refuses. The check and the name read `projects` in
  SQL (`ProjectName`, and a `LEFT JOIN` on every read), never write it; so a renamed project shows
  its new name, and a deleted one leaves its tasks with no project (`ON DELETE SET NULL`).
- **`workspace` in a PATCH is ignored**: a task isn't moved, as the mock's `taskFrom` ignores it.
- **Create** puts the task in the caller's first workspace when none is named (the mock uses its
  first workspace), to do unless a `status` is sent; the title is trimmed. A caller in no workspace
  at all is answered 404, as for a workspace that isn't theirs (the mock falls back to `w-acme`).
- **Words** are the mock's: "A task needs a title.", "Pick a project in this workspace.", "That
  file didn't arrive; add it again.".
- **Ids** are UUIDs made by Postgres, not the mock's `t-<title>` slugs; the client only echoes them.

## Not done

- **Attachments.** `POST /files` (uploadFile) isn't built, so no file id can exist: any
  `attachment` id is 422 on `attachment`, as the mock refuses a file it never got; `null` is
  accepted and changes nothing. A task is never answered with an `attachment`. Files are their own
  feature.
- **A day the calendar doesn't have** (`"2026-11-31"`) passes the contract's `format: date` check
  and fails when the generated body is decoded, so it is 400 `invalid` with no `field` (the client
  never sends one: its picker makes days). Naming the field means reading the decode error in
  `internal/server`; left for a change of its own.
- **A project deleted between the check and the save** fails the foreign key: a logged 500, not
  422 on `project`. Rare; mapping SQLSTATE 23503 in the store is a change of its own.
- **Lookups by id** compare `id::text`, as projects does, so a malformed id is 404 rather than a
  cast error; that keeps the primary key's index out of those reads. `tasks_project` still serves
  the foreign key when a project is deleted. Worth one fix for every feature, not just this one.
- **Seed**: `make seed` makes no tasks; the people can make them from the app.
