-- name: UserByEmail :one
SELECT id, name, email, password_hash, role FROM users WHERE lower(email) = lower(@email);

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, refresh_hash, user_id, ends_at, refresh_ends_at)
VALUES (@token_hash, @refresh_hash, @user_id, @ends_at, @refresh_ends_at);

-- name: SessionPerson :one
-- The person a live session names, with the workspaces they belong to.
SELECT u.id, u.name, u.email, u.role, s.ends_at,
       COALESCE(array_agg(m.workspace_id ORDER BY m.workspace_id) FILTER (WHERE m.workspace_id IS NOT NULL), '{}')::text[] AS workspaces
FROM sessions s
JOIN users u ON u.id = s.user_id
LEFT JOIN memberships m ON m.user_id = u.id
WHERE s.token_hash = @token_hash AND s.ends_at > now()
GROUP BY u.id, s.ends_at;

-- name: RefreshSession :one
-- A new pair of tokens for the session the refresh token names, while it lasts.
UPDATE sessions
SET token_hash = @new_token_hash, refresh_hash = @new_refresh_hash, ends_at = @ends_at, refresh_ends_at = @refresh_ends_at
WHERE refresh_hash = @refresh_hash AND refresh_ends_at > now()
RETURNING user_id;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = @token_hash;
