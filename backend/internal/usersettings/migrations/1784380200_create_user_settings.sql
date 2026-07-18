-- +migrate Up
CREATE TABLE user_settings (
    user_id VARCHAR PRIMARY KEY,
    timezone TEXT NOT NULL,
    agent_model TEXT NOT NULL,
    version BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_modified_by VARCHAR NOT NULL,
    CONSTRAINT user_settings_timezone_not_blank CHECK (LENGTH(BTRIM(timezone)) > 0),
    CONSTRAINT user_settings_agent_model_not_blank CHECK (LENGTH(BTRIM(agent_model)) > 0),
    CONSTRAINT user_settings_version_positive CHECK (version > 0)
);

-- +migrate Down
DROP TABLE user_settings;
