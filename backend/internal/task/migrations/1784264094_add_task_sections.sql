-- +migrate Up
ALTER TABLE tasks
	ADD COLUMN IF NOT EXISTS section_id VARCHAR(255);

DO $$
BEGIN
	IF TO_REGCLASS('project_sections') IS NOT NULL AND NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'tasks_section_id_fkey'
			AND conrelid = 'tasks'::REGCLASS
	) THEN
		ALTER TABLE tasks
			ADD CONSTRAINT tasks_section_id_fkey
			FOREIGN KEY (section_id) REFERENCES project_sections (id) ON DELETE SET NULL;
	END IF;
END
$$;

CREATE INDEX tasks_project_section_position_idx
	ON tasks (user_id, project_id, section_id, position)
	WHERE parent_id IS NULL;

-- +migrate Down
DROP INDEX tasks_project_section_position_idx;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_section_id_fkey;
ALTER TABLE tasks DROP COLUMN section_id;
