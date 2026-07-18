-- +migrate Up
ALTER TABLE activity_events
	ADD COLUMN stream_offset BIGSERIAL;

CREATE UNIQUE INDEX activity_events_stream_offset_idx
	ON activity_events (stream_offset);

-- +migrate Down
DROP INDEX activity_events_stream_offset_idx;

ALTER TABLE activity_events
	DROP COLUMN stream_offset;
