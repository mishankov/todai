-- +migrate Up
ALTER TABLE agent_messages
	ADD COLUMN context_type VARCHAR(32),
	ADD COLUMN context_id VARCHAR(255),
	ADD COLUMN context_action VARCHAR(32),
	ADD CONSTRAINT agent_messages_context_valid CHECK (
		(context_type IS NULL AND context_id IS NULL AND context_action IS NULL) OR
		(context_type = 'task' AND LENGTH(BTRIM(context_id)) > 0 AND context_action = 'decompose')
	);

-- +migrate Down
ALTER TABLE agent_messages
	DROP CONSTRAINT agent_messages_context_valid,
	DROP COLUMN context_action,
	DROP COLUMN context_id,
	DROP COLUMN context_type;
