-- A task is read with its project's name (projects is read to show it,
-- never written). The filters a list takes, the same in ListTasks and
-- CountTasks: the person's workspaces (or the one asked), words in the
-- title or the project's name (strpos: what was typed is never a pattern),
-- a status, the done ones left out, and a project.

-- name: ListTasks :many
SELECT t.id, t.workspace_id, t.title, t.status, t.due, t.updated_at,
       coalesce(p.id::text, '')::text AS project_id, coalesce(p.name, '')::text AS project_name
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
WHERE t.workspace_id = ANY (@workspaces::text[])
  AND (@q::text = '' OR strpos(lower(t.title || ' ' || coalesce(p.name, '')), lower(@q)) > 0)
  AND (@status::text = '' OR t.status = @status)
  AND (NOT @hide_done::bool OR t.status <> 'done')
  AND (@project::text = '' OR t.project_id::text = @project)
ORDER BY
  CASE WHEN @sort::text = 'title' AND NOT @descending::bool THEN lower(t.title) END ASC,
  CASE WHEN @sort = 'title' AND @descending THEN lower(t.title) END DESC,
  CASE WHEN @sort = 'due' AND NOT @descending THEN t.due END ASC NULLS LAST,
  CASE WHEN @sort = 'due' AND @descending THEN t.due END DESC NULLS FIRST,
  CASE WHEN @sort = 'updated' AND NOT @descending THEN t.updated_at END ASC,
  t.updated_at DESC,
  t.id
LIMIT @lim OFFSET @off;

-- name: CountTasks :one
-- Every match, and those of them past their day (before @today) and not done.
SELECT count(*)::int AS total,
       (count(*) FILTER (WHERE t.due < @today::date AND t.status <> 'done'))::int AS late
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
WHERE t.workspace_id = ANY (@workspaces::text[])
  AND (@q::text = '' OR strpos(lower(t.title || ' ' || coalesce(p.name, '')), lower(@q)) > 0)
  AND (@status::text = '' OR t.status = @status)
  AND (NOT @hide_done::bool OR t.status <> 'done')
  AND (@project::text = '' OR t.project_id::text = @project);

-- name: Task :one
SELECT t.id, t.workspace_id, t.title, t.status, t.due, t.updated_at,
       coalesce(p.id::text, '')::text AS project_id, coalesce(p.name, '')::text AS project_name
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
WHERE t.id::text = @id::text;

-- name: ProjectName :one
SELECT name FROM projects WHERE id::text = @id::text AND workspace_id = @workspace_id;

-- name: CreateTask :one
INSERT INTO tasks (workspace_id, title, status, due, project_id)
VALUES (@workspace_id, @title, @status, sqlc.narg(due), NULLIF(@project_id::text, '')::uuid)
RETURNING id;

-- name: UpdateTask :one
UPDATE tasks
SET title = @title, status = @status, due = sqlc.narg(due), project_id = NULLIF(@project_id::text, '')::uuid, updated_at = now()
WHERE id::text = @id::text
RETURNING id;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id::text = @id::text;
