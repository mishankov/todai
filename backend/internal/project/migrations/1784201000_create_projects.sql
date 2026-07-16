-- +migrate Up
CREATE TABLE projects (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    name TEXT NOT NULL,
    position BIGINT NOT NULL,
    version BIGINT NOT NULL,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_modified_by VARCHAR NOT NULL,
    CONSTRAINT projects_name_not_blank CHECK (LENGTH(BTRIM(name)) > 0),
    CONSTRAINT projects_version_positive CHECK (version > 0)
);

CREATE INDEX projects_user_position_idx
    ON projects (user_id, position)
    WHERE archived_at IS NULL;

-- +migrate Down
DROP TABLE projects;
