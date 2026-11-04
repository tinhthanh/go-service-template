-- Files a person sends, kept as they came, and a task's attachment. A file is
-- its sender's until a task names it; a task may name only its maker's own
-- (the service checks). Deleting a file leaves the task, with none.
CREATE TABLE files (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT NOT NULL CHECK (name <> ''),
    type       TEXT NOT NULL,
    size       INTEGER NOT NULL CHECK (size >= 0),
    data       BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX files_owner ON files (owner_id);

ALTER TABLE tasks ADD COLUMN attachment_id UUID REFERENCES files (id) ON DELETE SET NULL;
