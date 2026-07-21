-- +migrate Up
ALTER TABLE agent_sessions
	ADD COLUMN kind VARCHAR(32) NOT NULL DEFAULT 'conversation',
	ADD CONSTRAINT agent_sessions_kind_valid CHECK (kind IN ('conversation', 'action'));

ALTER TABLE agent_runs
	ADD COLUMN kind VARCHAR(32) NOT NULL DEFAULT 'conversation',
	ADD COLUMN context_type VARCHAR(32),
	ADD COLUMN context_id VARCHAR(255),
	ADD COLUMN context_action VARCHAR(32),
	ADD CONSTRAINT agent_runs_kind_context_valid CHECK (
		(kind = 'conversation' AND context_type IS NULL AND context_id IS NULL AND context_action IS NULL) OR
		(kind = 'action' AND context_type = 'task' AND LENGTH(BTRIM(context_id)) > 0 AND context_action = 'decompose')
	);

UPDATE agent_runs AS run
SET kind = 'action',
	context_type = message.context_type,
	context_id = message.context_id,
	context_action = message.context_action
FROM agent_messages AS message
WHERE message.run_id = run.id
	AND message.role = 'user'
	AND message.context_type IS NOT NULL;

-- +migrate Down
ALTER TABLE agent_runs
	DROP CONSTRAINT agent_runs_kind_context_valid,
	DROP COLUMN context_action,
	DROP COLUMN context_id,
	DROP COLUMN context_type,
	DROP COLUMN kind;

ALTER TABLE agent_sessions
	DROP CONSTRAINT agent_sessions_kind_valid,
	DROP COLUMN kind;
