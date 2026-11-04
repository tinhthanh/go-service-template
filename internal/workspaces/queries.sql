-- name: WorkspacesOf :many
SELECT w.id, w.name FROM workspaces w JOIN memberships m ON m.workspace_id = w.id
WHERE m.user_id = @user_id ORDER BY lower(w.name), w.id;

-- name: WorkspaceIDTaken :one
SELECT EXISTS (SELECT 1 FROM workspaces WHERE id = @id);

-- name: CreateWorkspace :exec
INSERT INTO workspaces (id, name) VALUES (@id, @name);

-- name: Join :exec
INSERT INTO memberships (user_id, workspace_id) VALUES (@user_id, @workspace_id);
