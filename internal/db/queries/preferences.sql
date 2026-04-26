-- name: GetPreference :one
SELECT value FROM preferences WHERE key = ?;

-- name: SetPreference :exec
INSERT INTO preferences (key, value)
VALUES (?, ?)
ON CONFLICT (key) DO UPDATE SET
    value      = excluded.value,
    updated_at = CURRENT_TIMESTAMP;

-- name: DeletePreference :exec
DELETE FROM preferences WHERE key = ?;
