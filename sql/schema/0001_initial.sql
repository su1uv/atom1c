-- +goose Up
-- User records were never used by the application. Keep this migration version
-- as a no-op so development databases with version 1 applied remain readable.
SELECT 1;

-- +goose Down
SELECT 1;
