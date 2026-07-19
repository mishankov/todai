-- +migrate Up
CREATE TABLE task_comments (
	id VARCHAR(255) PRIMARY KEY,
	task_id VARCHAR(255) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	user_id VARCHAR(255) NOT NULL,
	author_id VARCHAR(255) NOT NULL,
	body TEXT NOT NULL,
	version BIGINT NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_modified_by VARCHAR(255) NOT NULL,
	CONSTRAINT task_comments_body_not_blank CHECK (LENGTH(BTRIM(body)) > 0),
	CONSTRAINT task_comments_version_positive CHECK (version > 0)
);

CREATE INDEX task_comments_task_time_idx
	ON task_comments (task_id, created_at, id);

CREATE INDEX tasks_parent_position_idx
	ON tasks (user_id, parent_id, position)
	WHERE parent_id IS NOT NULL;

-- +migrate Down
DROP INDEX tasks_parent_position_idx;
DROP TABLE task_comments;
