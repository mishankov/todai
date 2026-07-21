-- +migrate Up
ALTER TABLE agent_runs
	ADD COLUMN project_id VARCHAR(255),
	ADD CONSTRAINT agent_runs_project_not_blank CHECK (
		project_id IS NULL OR LENGTH(BTRIM(project_id)) > 0
	);

CREATE INDEX agent_runs_project_time_idx
	ON agent_runs (user_id, project_id, created_at DESC, id DESC);

-- Existing runs are intentionally left unscoped because their project cannot be inferred safely.
-- All runs created by the updated application have a non-null project_id.

-- +migrate Down
DROP INDEX agent_runs_project_time_idx;

ALTER TABLE agent_runs
	DROP CONSTRAINT agent_runs_project_not_blank,
	DROP COLUMN project_id;
