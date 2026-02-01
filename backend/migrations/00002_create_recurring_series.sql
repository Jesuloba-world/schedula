-- +goose Up
CREATE TABLE IF NOT EXISTS recurring_series (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    notes TEXT NULL,
    timezone TEXT NOT NULL,
    dtstart TIMESTAMPTZ NOT NULL,
    duration_seconds INTEGER NOT NULL,
    frequency TEXT NOT NULL,
    interval INTEGER NOT NULL DEFAULT 1,
    byweekday SMALLINT[] NOT NULL,
    until TIMESTAMPTZ NULL,
    count INTEGER NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS recurring_series_user_id_idx ON recurring_series (user_id);

CREATE TABLE IF NOT EXISTS recurring_exceptions (
    id UUID PRIMARY KEY,
    series_id UUID NOT NULL REFERENCES recurring_series (id) ON DELETE CASCADE,
    occurrence_start TIMESTAMPTZ NOT NULL,
    kind TEXT NOT NULL,
    override_start TIMESTAMPTZ NULL,
    override_end TIMESTAMPTZ NULL,
    override_title TEXT NULL,
    override_notes TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE recurring_exceptions
ADD CONSTRAINT recurring_exceptions_kind_check CHECK (kind IN ('skip', 'override'));

CREATE UNIQUE INDEX IF NOT EXISTS recurring_exceptions_series_occurrence_idx
ON recurring_exceptions (series_id, occurrence_start);

CREATE INDEX IF NOT EXISTS recurring_exceptions_series_id_idx
ON recurring_exceptions (series_id);

-- +goose Down
DROP TABLE IF EXISTS recurring_exceptions;
DROP TABLE IF EXISTS recurring_series;
