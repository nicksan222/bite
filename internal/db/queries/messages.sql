-- name: AppendMessage :one
INSERT INTO messages (conversation_id, role, content)
VALUES (?, ?, ?)
RETURNING *;

-- name: ListMessages :many
SELECT * FROM messages
WHERE conversation_id = ?
ORDER BY created_at ASC, id ASC;

-- name: CountMessages :one
SELECT COUNT(*) FROM messages
WHERE conversation_id = ?;
