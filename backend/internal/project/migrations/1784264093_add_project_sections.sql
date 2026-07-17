-- +migrate Up
ALTER TABLE projects
	ADD COLUMN layout VARCHAR(20) NOT NULL DEFAULT 'list',
	ADD CONSTRAINT projects_layout_valid CHECK (layout IN ('list', 'board'));

CREATE TABLE project_sections (
	id VARCHAR(255) PRIMARY KEY,
	user_id VARCHAR(255) NOT NULL,
	project_id VARCHAR(255) NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	position BIGINT NOT NULL,
	version BIGINT NOT NULL DEFAULT 1,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	last_modified_by VARCHAR(255) NOT NULL,
	CONSTRAINT project_sections_name_not_blank CHECK (LENGTH(BTRIM(name)) > 0),
	CONSTRAINT project_sections_version_positive CHECK (version > 0)
);

CREATE INDEX project_sections_project_position_idx
	ON project_sections (user_id, project_id, position);

DO $$
BEGIN
	IF TO_REGCLASS('tasks') IS NOT NULL THEN
		ALTER TABLE tasks ADD COLUMN IF NOT EXISTS section_id VARCHAR(255);
		IF NOT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = 'tasks_section_id_fkey'
				AND conrelid = 'tasks'::REGCLASS
		) THEN
			ALTER TABLE tasks
				ADD CONSTRAINT tasks_section_id_fkey
				FOREIGN KEY (section_id) REFERENCES project_sections (id) ON DELETE SET NULL;
		END IF;
	END IF;
END
$$;

-- +migrate Down
ALTER TABLE IF EXISTS tasks DROP CONSTRAINT IF EXISTS tasks_section_id_fkey;
DROP TABLE project_sections;
ALTER TABLE projects DROP COLUMN layout;
