-- +migrate Up
ALTER TABLE user_settings
	ADD COLUMN appearance TEXT NOT NULL DEFAULT 'system',
	ADD CONSTRAINT user_settings_appearance_valid CHECK (
		appearance IN ('system', 'light', 'dark')
	);

-- +migrate Down
ALTER TABLE user_settings
	DROP CONSTRAINT user_settings_appearance_valid,
	DROP COLUMN appearance;
