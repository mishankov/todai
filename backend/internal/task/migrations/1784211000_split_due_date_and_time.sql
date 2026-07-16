-- +migrate Up
ALTER TABLE tasks
	ADD COLUMN due_date DATE,
	ADD COLUMN due_time TIME(0);

UPDATE tasks
SET due_date = (due_at AT TIME ZONE COALESCE(NULLIF(due_timezone, ''), 'UTC'))::DATE,
	due_time = date_trunc(
		'minute',
		due_at AT TIME ZONE COALESCE(NULLIF(due_timezone, ''), 'UTC')
	)::TIME(0)
WHERE due_at IS NOT NULL;

UPDATE tasks SET due_timezone = NULL WHERE due_time IS NULL;

ALTER TABLE tasks
	DROP COLUMN due_at,
	ADD CONSTRAINT tasks_due_time_requires_date CHECK (due_time IS NULL OR due_date IS NOT NULL),
	ADD CONSTRAINT tasks_due_timezone_requires_time CHECK (due_timezone IS NULL OR due_time IS NOT NULL);

-- +migrate Down
ALTER TABLE tasks ADD COLUMN due_at TIMESTAMPTZ;

UPDATE tasks
SET due_at = CASE
	WHEN due_date IS NULL THEN NULL
	WHEN due_time IS NULL THEN due_date::TIMESTAMP AT TIME ZONE 'UTC'
	ELSE (due_date + due_time) AT TIME ZONE COALESCE(NULLIF(due_timezone, ''), 'UTC')
END;

ALTER TABLE tasks
	DROP CONSTRAINT tasks_due_timezone_requires_time,
	DROP CONSTRAINT tasks_due_time_requires_date,
	DROP COLUMN due_time,
	DROP COLUMN due_date;
