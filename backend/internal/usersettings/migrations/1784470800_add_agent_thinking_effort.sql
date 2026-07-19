-- +migrate Up
ALTER TABLE user_settings
	ADD COLUMN agent_thinking_effort TEXT NOT NULL DEFAULT 'medium',
	ADD CONSTRAINT user_settings_agent_thinking_effort_valid CHECK (
		agent_thinking_effort IN ('off', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max')
	);

-- +migrate Down
ALTER TABLE user_settings
	DROP CONSTRAINT user_settings_agent_thinking_effort_valid,
	DROP COLUMN agent_thinking_effort;
