-- +migrate Up
CREATE TABLE agent_sessions (
	id VARCHAR(255) PRIMARY KEY,
	user_id VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX agent_sessions_user_time_idx
	ON agent_sessions (user_id, updated_at DESC, id DESC);

CREATE TABLE agent_runs (
	id VARCHAR(255) PRIMARY KEY,
	session_id VARCHAR(255) NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
	user_id VARCHAR(255) NOT NULL,
	status VARCHAR(32) NOT NULL,
	correlation_id VARCHAR(255) NOT NULL,
	started_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ,
	error TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT agent_runs_status_valid CHECK (
		status IN ('queued', 'running', 'completed', 'failed', 'aborted')
	),
	CONSTRAINT agent_runs_correlation_not_blank CHECK (LENGTH(BTRIM(correlation_id)) > 0)
);

CREATE INDEX agent_runs_session_time_idx
	ON agent_runs (session_id, created_at DESC, id DESC);

CREATE INDEX agent_runs_user_status_idx
	ON agent_runs (user_id, status, created_at DESC);

CREATE TABLE agent_messages (
	id VARCHAR(255) PRIMARY KEY,
	session_id VARCHAR(255) NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
	run_id VARCHAR(255) REFERENCES agent_runs(id) ON DELETE CASCADE,
	role VARCHAR(32) NOT NULL,
	content TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT agent_messages_role_valid CHECK (role IN ('user', 'assistant')),
	CONSTRAINT agent_messages_content_not_blank CHECK (
		role = 'assistant' OR LENGTH(BTRIM(content)) > 0
	)
);

CREATE UNIQUE INDEX agent_messages_assistant_run_idx
	ON agent_messages (run_id, role)
	WHERE role = 'assistant';

CREATE INDEX agent_messages_session_time_idx
	ON agent_messages (session_id, created_at, id);

CREATE TABLE agent_run_events (
	stream_offset BIGSERIAL PRIMARY KEY,
	run_id VARCHAR(255) NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
	session_id VARCHAR(255) NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
	runtime_sequence BIGINT NOT NULL,
	type VARCHAR(255) NOT NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	payload JSONB NOT NULL,
	CONSTRAINT agent_run_events_sequence_positive CHECK (runtime_sequence > 0),
	CONSTRAINT agent_run_events_type_not_blank CHECK (LENGTH(BTRIM(type)) > 0),
	CONSTRAINT agent_run_events_payload_object CHECK (JSONB_TYPEOF(payload) = 'object'),
	CONSTRAINT agent_run_events_run_sequence_unique UNIQUE (run_id, runtime_sequence)
);

CREATE INDEX agent_run_events_session_stream_idx
	ON agent_run_events (session_id, stream_offset);

-- +migrate Down
DROP TABLE agent_run_events;
DROP TABLE agent_messages;
DROP TABLE agent_runs;
DROP TABLE agent_sessions;
