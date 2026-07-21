-- +migrate Up
DO $$
DECLARE
	affected_user RECORD;
	personal_project_id VARCHAR(255);
	candidate_suffix INTEGER;
	project_position BIGINT;
BEGIN
	IF TO_REGCLASS('projects') IS NOT NULL THEN
		FOR affected_user IN
			SELECT DISTINCT candidate.user_id
			FROM tasks AS candidate
			WHERE candidate.project_id IS NULL
				OR NOT EXISTS (SELECT 1 FROM projects WHERE id = candidate.project_id)
		LOOP
			candidate_suffix := 0;
			personal_project_id := 'legacy-personal-' || MD5(affected_user.user_id);
			WHILE EXISTS (SELECT 1 FROM projects WHERE id = personal_project_id) LOOP
				candidate_suffix := candidate_suffix + 1;
				personal_project_id := 'legacy-personal-' ||
					MD5(affected_user.user_id || ':' || candidate_suffix::TEXT);
			END LOOP;

			SELECT COALESCE(MAX(position), 0) + 1024
			INTO project_position
			FROM projects
			WHERE user_id = affected_user.user_id AND archived_at IS NULL;

			INSERT INTO projects (
				id, user_id, name, layout, color_theme, agent_model, agent_thinking_effort,
				position, version, created_at, updated_at, last_modified_by
			)
			VALUES (
				personal_project_id, affected_user.user_id, 'Personal', 'list', 'sage', '', 'medium',
				project_position, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'migration'
			);

			UPDATE tasks
			SET project_id = personal_project_id,
				section_id = NULL
			WHERE user_id = affected_user.user_id
				AND (
					project_id IS NULL
					OR NOT EXISTS (SELECT 1 FROM projects WHERE id = tasks.project_id)
				);
		END LOOP;

		IF TO_REGCLASS('user_settings') IS NOT NULL THEN
			EXECUTE '
				UPDATE projects AS project
				SET agent_model = settings.agent_model,
					agent_thinking_effort = settings.agent_thinking_effort
				FROM user_settings AS settings
				WHERE project.id LIKE ''legacy-personal-%''
					AND settings.user_id = project.user_id
			';
		END IF;

		IF NOT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = 'tasks_project_id_fkey' AND conrelid = 'tasks'::REGCLASS
		) THEN
			ALTER TABLE tasks
				ADD CONSTRAINT tasks_project_id_fkey
				FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE;
		END IF;
		ALTER TABLE tasks ALTER COLUMN project_id SET NOT NULL;

		IF TO_REGCLASS('activity_events') IS NOT NULL AND EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'activity_events' AND column_name = 'project_id'
		) THEN
			UPDATE activity_events AS event
			SET project_id = parent.project_id
			FROM tasks AS parent
			WHERE event.project_id IS NULL
				AND event.aggregate_type = 'task'
				AND parent.id = event.aggregate_id
				AND parent.user_id = event.user_id;
			IF TO_REGCLASS('task_comments') IS NOT NULL THEN
				UPDATE activity_events AS event
				SET project_id = parent.project_id
				FROM task_comments AS comment
				JOIN tasks AS parent
					ON parent.id = comment.task_id AND parent.user_id = comment.user_id
				WHERE event.project_id IS NULL
					AND event.aggregate_type = 'task_comment'
					AND comment.id = event.aggregate_id
					AND comment.user_id = event.user_id;
			END IF;
		END IF;
	END IF;
END
$$;

DROP INDEX IF EXISTS tasks_inbox_idx;
CREATE INDEX IF NOT EXISTS tasks_project_inbox_idx
	ON tasks (user_id, project_id, status, position)
	WHERE section_id IS NULL AND parent_id IS NULL;

-- +migrate Down
DROP INDEX IF EXISTS tasks_project_inbox_idx;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_project_id_fkey;
ALTER TABLE tasks ALTER COLUMN project_id DROP NOT NULL;
CREATE INDEX IF NOT EXISTS tasks_inbox_idx
	ON tasks (user_id, status, position)
	WHERE project_id IS NULL AND parent_id IS NULL;
