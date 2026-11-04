-- What a search finds, ranked as the client's lib/search-rank.ts ranks it:
-- a name the words are (100), one that starts with them (80), one with a
-- word that starts with them (60), one that has them anywhere (40), then a
-- match in the rest (20); ties by name. Each kind at most @per, the caller's
-- workspaces only. Projects and tasks are read to find them, never written;
-- what was typed is compared with strpos, never as a pattern.

-- name: SearchProjects :many
SELECT id, name, owner AS detail, status, score::int AS score FROM (
  SELECT id, name, owner, status,
    CASE
      WHEN lower(name) = lower(@q::text) THEN 100
      WHEN strpos(lower(name), lower(@q)) = 1 THEN 80
      WHEN strpos(' ' || regexp_replace(lower(name), '[\s_/-]+', ' ', 'g'), ' ' || lower(@q)) > 0 THEN 60
      WHEN strpos(lower(name), lower(@q)) > 0 THEN 40
      WHEN strpos(lower(description || ' ' || owner), lower(@q)) > 0 THEN 20
      ELSE 0
    END AS score
  FROM projects WHERE workspace_id = ANY (@workspaces::text[])
) found
WHERE score > 0
ORDER BY score DESC, lower(name), id
LIMIT @per;

-- name: SearchTasks :many
SELECT id, name, detail, status, score::int AS score FROM (
  SELECT t.id, t.title AS name, coalesce(p.name, '')::text AS detail, t.status,
    CASE
      WHEN lower(t.title) = lower(@q::text) THEN 100
      WHEN strpos(lower(t.title), lower(@q)) = 1 THEN 80
      WHEN strpos(' ' || regexp_replace(lower(t.title), '[\s_/-]+', ' ', 'g'), ' ' || lower(@q)) > 0 THEN 60
      WHEN strpos(lower(t.title), lower(@q)) > 0 THEN 40
      WHEN strpos(lower(coalesce(p.name, '')), lower(@q)) > 0 THEN 20
      ELSE 0
    END AS score
  FROM tasks t LEFT JOIN projects p ON p.id = t.project_id
  WHERE t.workspace_id = ANY (@workspaces::text[])
) found
WHERE score > 0
ORDER BY score DESC, lower(name), id
LIMIT @per;
