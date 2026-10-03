-- +goose Up
-- Older versions wrote Go time.Time.String() values, including local offsets,
-- fractional seconds, and sometimes a monotonic-clock suffix. Normalize those
-- values to UTC SQLite CURRENT_TIMESTAMP format before SQL code relies on
-- lexicographic ordering. This deliberately truncates subsecond precision.
CREATE TEMP TABLE atom1c_timestamp_map (
    old_value TEXT PRIMARY KEY,
    new_value TEXT NOT NULL
) STRICT;

WITH timestamp_values(value) AS (
    SELECT created_at FROM users
    UNION ALL SELECT updated_at FROM users
    UNION ALL SELECT created_at FROM feeds
    UNION ALL SELECT updated_at FROM feeds
    UNION ALL SELECT last_fetched_at FROM feeds
), legacy_values AS (
    SELECT DISTINCT
        value,
        CASE
            WHEN instr(substr(value, 20), ' -') > 0
                THEN instr(substr(value, 20), ' -')
            ELSE instr(substr(value, 20), ' +')
        END AS offset_pos
    FROM timestamp_values
    WHERE instr(substr(value, 20), ' -') > 0
       OR instr(substr(value, 20), ' +') > 0
)
INSERT INTO atom1c_timestamp_map (old_value, new_value)
SELECT
    value,
    strftime(
        '%Y-%m-%d %H:%M:%S',
        substr(value, 1, 19)
            || substr(value, 20, offset_pos - 1)
            || substr(value, 20 + offset_pos, 3)
            || ':'
            || substr(value, 23 + offset_pos, 2)
    )
FROM legacy_values;

UPDATE users
SET created_at = (
    SELECT new_value FROM atom1c_timestamp_map WHERE old_value = users.created_at
)
WHERE created_at IN (SELECT old_value FROM atom1c_timestamp_map);

UPDATE users
SET updated_at = (
    SELECT new_value FROM atom1c_timestamp_map WHERE old_value = users.updated_at
)
WHERE updated_at IN (SELECT old_value FROM atom1c_timestamp_map);

UPDATE feeds
SET created_at = (
    SELECT new_value FROM atom1c_timestamp_map WHERE old_value = feeds.created_at
)
WHERE created_at IN (SELECT old_value FROM atom1c_timestamp_map);

UPDATE feeds
SET updated_at = (
    SELECT new_value FROM atom1c_timestamp_map WHERE old_value = feeds.updated_at
)
WHERE updated_at IN (SELECT old_value FROM atom1c_timestamp_map);

UPDATE feeds
SET last_fetched_at = (
    SELECT new_value FROM atom1c_timestamp_map WHERE old_value = feeds.last_fetched_at
)
WHERE last_fetched_at IN (SELECT old_value FROM atom1c_timestamp_map);

-- Fail the migration instead of silently retaining an unknown timestamp format.
CREATE TEMP TABLE atom1c_timestamp_check (
    value TEXT NOT NULL CHECK (value = 'ok')
) STRICT;

WITH timestamp_values(value) AS (
    SELECT created_at FROM users
    UNION ALL SELECT updated_at FROM users
    UNION ALL SELECT created_at FROM feeds
    UNION ALL SELECT updated_at FROM feeds
    UNION ALL SELECT last_fetched_at FROM feeds
)
INSERT INTO atom1c_timestamp_check (value)
SELECT CASE
    WHEN value IS NULL THEN 'ok'
    WHEN value = strftime('%Y-%m-%d %H:%M:%S', value) THEN 'ok'
    ELSE 'invalid'
END
FROM timestamp_values;

DROP TABLE atom1c_timestamp_check;
DROP TABLE atom1c_timestamp_map;

-- +goose Down
-- Normalization is intentionally irreversible: original offsets and
-- subsecond precision cannot be reconstructed.
SELECT 1;
