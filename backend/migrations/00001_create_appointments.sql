-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS appointments (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    notes TEXT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE appointments
ADD CONSTRAINT appointments_valid_time_range CHECK (end_time > start_time);

ALTER TABLE appointments
ADD CONSTRAINT appointments_no_overlap EXCLUDE USING gist (
    user_id
    WITH
        =,
        tstzrange (start_time, end_time, '[)')
    WITH
        &&
);

CREATE INDEX IF NOT EXISTS appointments_user_start_time_idx ON appointments (user_id, start_time);

CREATE INDEX IF NOT EXISTS appointments_user_end_time_idx ON appointments (user_id, end_time);

-- +goose Down
DROP TABLE IF EXISTS appointments;
