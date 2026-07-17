-- +migrate Up
CREATE TABLE activity_events (
	id VARCHAR(255) PRIMARY KEY,
	user_id VARCHAR(255),
	type VARCHAR(255) NOT NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	actor_type VARCHAR(32) NOT NULL,
	actor_id VARCHAR(255),
	source VARCHAR(100) NOT NULL,
	aggregate_type VARCHAR(100),
	aggregate_id VARCHAR(255),
	correlation_id VARCHAR(255) NOT NULL,
	agent_run_id VARCHAR(255),
	payload JSONB NOT NULL,
	CONSTRAINT activity_events_type_not_blank CHECK (LENGTH(BTRIM(type)) > 0),
	CONSTRAINT activity_events_actor_type_valid CHECK (
		actor_type IN ('user', 'built_in_agent', 'external_agent', 'system')
	),
	CONSTRAINT activity_events_source_not_blank CHECK (LENGTH(BTRIM(source)) > 0),
	CONSTRAINT activity_events_correlation_not_blank CHECK (
		LENGTH(BTRIM(correlation_id)) > 0
	),
	CONSTRAINT activity_events_aggregate_complete CHECK (
		(aggregate_type IS NULL AND aggregate_id IS NULL)
		OR (aggregate_type IS NOT NULL AND aggregate_id IS NOT NULL)
	),
	CONSTRAINT activity_events_payload_object CHECK (JSONB_TYPEOF(payload) = 'object')
);

CREATE INDEX activity_events_user_time_idx
	ON activity_events (user_id, occurred_at DESC, id DESC);

CREATE INDEX activity_events_aggregate_idx
	ON activity_events (user_id, aggregate_type, aggregate_id, occurred_at DESC)
	WHERE aggregate_id IS NOT NULL;

CREATE INDEX activity_events_correlation_idx
	ON activity_events (correlation_id, occurred_at, id);

CREATE INDEX activity_events_agent_run_idx
	ON activity_events (agent_run_id, occurred_at, id)
	WHERE agent_run_id IS NOT NULL;

-- +migrate Down
DROP TABLE activity_events;
