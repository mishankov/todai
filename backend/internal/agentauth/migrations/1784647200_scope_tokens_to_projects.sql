-- +migrate Up
DELETE FROM agent_tokens;

ALTER TABLE agent_tokens
	ADD COLUMN project_id VARCHAR(255) NOT NULL,
	ADD CONSTRAINT agent_tokens_project_not_blank CHECK (LENGTH(BTRIM(project_id)) > 0);

CREATE INDEX agent_tokens_project_idx ON agent_tokens (user_id, project_id, expires_at);

-- +migrate Down
DROP INDEX agent_tokens_project_idx;

ALTER TABLE agent_tokens
	DROP CONSTRAINT agent_tokens_project_not_blank,
	DROP COLUMN project_id;
