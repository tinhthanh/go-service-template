-- People, the workspaces they belong to, and their sessions. A session is a
-- random token in an httpOnly cookie; only its hash is kept here, so the
-- table read is no session taken.
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('owner', 'member', 'viewer')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email ON users (lower(email));

CREATE TABLE workspaces (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, workspace_id)
);

CREATE TABLE sessions (
    token_hash         BYTEA PRIMARY KEY,
    refresh_hash       BYTEA NOT NULL UNIQUE,
    user_id            UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ends_at            TIMESTAMPTZ NOT NULL,
    refresh_ends_at    TIMESTAMPTZ NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user ON sessions (user_id);
