-- The filters a list takes, the same in ListProjects and CountProjects:
-- the person's workspaces (or the one asked), words in the name,
-- description or owner (strpos: what was typed is never a pattern), a
-- status, and every tag asked.

-- name: ListProjects :many
SELECT *
FROM projects
WHERE workspace_id = ANY (@workspaces::text[])
  AND (@q::text = '' OR strpos(lower(name || ' ' || description || ' ' || owner), lower(@q)) > 0)
  AND (@status::text = '' OR status = @status)
  AND tags @> @tags::text[]
ORDER BY
  CASE WHEN @sort::text = 'name' AND NOT @descending::bool THEN lower(name) END ASC,
  CASE WHEN @sort = 'name' AND @descending THEN lower(name) END DESC,
  CASE WHEN @sort = 'owner' AND NOT @descending THEN lower(owner) END ASC,
  CASE WHEN @sort = 'owner' AND @descending THEN lower(owner) END DESC,
  CASE WHEN @sort = 'updated' AND NOT @descending THEN updated_at END ASC,
  updated_at DESC,
  id
LIMIT @lim OFFSET @off;

-- name: CountProjects :one
SELECT count(*)::int
FROM projects
WHERE workspace_id = ANY (@workspaces::text[])
  AND (@q::text = '' OR strpos(lower(name || ' ' || description || ' ' || owner), lower(@q)) > 0)
  AND (@status::text = '' OR status = @status)
  AND tags @> @tags::text[];

-- name: Project :one
SELECT * FROM projects WHERE id = @id;

-- name: NameTaken :one
SELECT EXISTS (SELECT 1 FROM projects WHERE workspace_id = @workspace_id AND lower(name) = lower(@name) AND id IS DISTINCT FROM sqlc.narg('other_than')::uuid);

-- name: CreateProject :one
INSERT INTO projects (workspace_id, name, description, owner)
VALUES (@workspace_id, @name, @description, @owner)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects SET name = @name, description = @description, status = @status, tags = @tags, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = @id;

-- name: Tags :many
SELECT DISTINCT unnest(tags)::text AS tag FROM projects WHERE workspace_id = ANY (@workspaces::text[]) ORDER BY 1;
