-- +migrate Up
ALTER TABLE activity_events
	ADD COLUMN project_id VARCHAR(255);

DO $$
BEGIN
	IF TO_REGCLASS('projects') IS NOT NULL THEN
		UPDATE activity_events AS event
		SET project_id = event.aggregate_id
		WHERE event.project_id IS NULL
			AND event.aggregate_type = 'project'
			AND EXISTS (
				SELECT 1 FROM projects
				WHERE id = event.aggregate_id AND user_id = event.user_id
			);
	END IF;

	IF TO_REGCLASS('project_sections') IS NOT NULL THEN
		UPDATE activity_events AS event
		SET project_id = section.project_id
		FROM project_sections AS section
		WHERE event.project_id IS NULL
			AND event.aggregate_type = 'section'
			AND section.id = event.aggregate_id
			AND section.user_id = event.user_id;
	END IF;

	IF TO_REGCLASS('tasks') IS NOT NULL THEN
		UPDATE activity_events AS event
		SET project_id = parent.project_id
		FROM tasks AS parent
		WHERE event.project_id IS NULL
			AND event.aggregate_type = 'task'
			AND parent.id = event.aggregate_id
			AND parent.user_id = event.user_id;
	END IF;

	IF TO_REGCLASS('task_comments') IS NOT NULL AND TO_REGCLASS('tasks') IS NOT NULL THEN
		UPDATE activity_events AS event
		SET project_id = parent.project_id
		FROM task_comments AS comment
		JOIN tasks AS parent ON parent.id = comment.task_id AND parent.user_id = comment.user_id
		WHERE event.project_id IS NULL
			AND event.aggregate_type = 'task_comment'
			AND comment.id = event.aggregate_id
			AND comment.user_id = event.user_id;
	END IF;
END
$$;

CREATE INDEX activity_events_project_stream_idx
	ON activity_events (user_id, project_id, stream_offset DESC)
	WHERE project_id IS NOT NULL;

-- +migrate Down
DROP INDEX activity_events_project_stream_idx;

ALTER TABLE activity_events
	DROP COLUMN project_id;
