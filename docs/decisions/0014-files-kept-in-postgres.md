# 0014: Files are kept in Postgres, up to 5 MB, their sender's

**Status:** accepted, 2026-11-04

**Context.** The client sends a file as it is (`POST /files`, its name URI-encoded in
`X-File-Name`, up to 5 MB: its `lib/upload.ts`), and a task names it by the id answered. The
contract has no download. A template's service should keep a file without a second system to run.

**Decision.** A file is a row: its sender, name, type, size and bytes (`bytea`), up to 5 MB
(`files.MaxSize`); more is 422 on `attachment`, as the client's mock answers. Every API request's
body is cut at 8 MB (`httpx.MaxBody`), a 413 past it, since the contract's validator reads a body
whole before a handler runs. A task names only a file its maker sent (`tasks.Store.AttachmentOf`
reads the files table to check, as ADR 0002 allows); a file a task already has stays when someone
else changes the task. Deleting a file leaves the task, with none.

**Consequences.** Backups and the database's size carry the files; 5 MB a file keeps that modest.
Moving them out is the files store's change alone: `files.Store` keeps the bytes, `AttachmentOf`
reads only the row.

**Revisit if.** Files grow past a few MB, are many, or need to be served: keep the bytes in object
storage (S3, R2) and the row here.
