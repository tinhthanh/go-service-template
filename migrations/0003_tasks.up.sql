-- A workspace's tasks. A task may name a project of its own workspace (the
-- service checks the workspace); when that project is deleted the task stays,
-- with no project. A due day is a day, with no time and no zone.
CREATE TABLE tasks (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    title        TEXT NOT NULL CHECK (title <> ''),
    status       TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'doing', 'done')),
    due          DATE,
    project_id   UUID REFERENCES projects (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tasks_updated ON tasks (workspace_id, updated_at DESC);
CREATE INDEX tasks_project ON tasks (project_id);
