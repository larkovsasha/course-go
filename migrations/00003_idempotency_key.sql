-- +goose Up
CREATE TABLE idempotency_keys (
    key         UUID PRIMARY KEY,
    request_hash TEXT NOT NULL,
    trip_id     UUID NOT NULL REFERENCES trips(id) DEFERRABLE INITIALLY DEFERRED,
    expires_at  TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS idempotency_keys;
