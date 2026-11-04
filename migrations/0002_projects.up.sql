-- A workspace's projects. A name is a workspace's own: two workspaces may
-- each have a "Shop", one may not have two in any case.
CREATE TABLE projects (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    owner        TEXT NOT NULL,
    tags         TEXT[] NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX projects_name ON projects (workspace_id, lower(name));
CREATE INDEX projects_updated ON projects (workspace_id, updated_at DESC);
CREATE INDEX projects_tags ON projects USING gin (tags);
