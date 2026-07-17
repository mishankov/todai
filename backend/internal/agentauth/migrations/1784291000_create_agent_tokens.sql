-- +migrate Up
CREATE TABLE agent_tokens (
	token_hash BYTEA PRIMARY KEY,
	user_id VARCHAR(255) NOT NULL,
	agent_session_id VARCHAR(255) NOT NULL,
	agent_run_id VARCHAR(255) NOT NULL,
	allowed_tools TEXT[] NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT agent_tokens_hash_length CHECK (OCTET_LENGTH(token_hash) = 32),
	CONSTRAINT agent_tokens_user_not_blank CHECK (LENGTH(BTRIM(user_id)) > 0),
	CONSTRAINT agent_tokens_session_not_blank CHECK (LENGTH(BTRIM(agent_session_id)) > 0),
	CONSTRAINT agent_tokens_run_not_blank CHECK (LENGTH(BTRIM(agent_run_id)) > 0),
	CONSTRAINT agent_tokens_tools_not_empty CHECK (CARDINALITY(allowed_tools) > 0),
	CONSTRAINT agent_tokens_tools_valid CHECK (
		allowed_tools <@ ARRAY[
			'task_get', 'task_search', 'project_list', 'view_query', 'task_create',
			'task_update', 'task_complete', 'task_reopen', 'task_move', 'task_reorder'
		]::TEXT[]
	)
);

CREATE INDEX agent_tokens_expiry_idx ON agent_tokens (expires_at);
CREATE INDEX agent_tokens_run_idx ON agent_tokens (agent_run_id, expires_at);

-- +migrate Down
DROP TABLE agent_tokens;
