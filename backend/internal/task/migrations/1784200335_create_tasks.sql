-- +migrate Up
CREATE TABLE tasks (
	id VARCHAR(255) PRIMARY KEY,
	user_id VARCHAR(255) NOT NULL,
	project_id VARCHAR(255),
	parent_id VARCHAR(255),
	title TEXT NOT NULL,
	description TEXT,
	status VARCHAR(20) NOT NULL CHECK (status IN ('active', 'completed')),
	priority SMALLINT NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 4),
	due_at TIMESTAMPTZ,
	due_timezone TEXT,
	position BIGINT NOT NULL,
	version BIGINT NOT NULL DEFAULT 1,
	completed_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	last_modified_by VARCHAR(255) NOT NULL,
	CONSTRAINT tasks_parent_fk FOREIGN KEY (parent_id) REFERENCES tasks (id) ON DELETE CASCADE
);

CREATE INDEX tasks_inbox_idx
	ON tasks (user_id, status, position)
	WHERE project_id IS NULL AND parent_id IS NULL;

-- +migrate Down
DROP TABLE tasks;
