-- name: CreateConversation :one
INSERT INTO conversations (title, model)
VALUES (?, ?)
RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations
WHERE id = ?
LIMIT 1;

-- name: ListConversations :many
SELECT * FROM conversations
ORDER BY updated_at DESC
LIMIT ?;

-- name: UpdateConversationTitle :exec
UPDATE conversations
SET title = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: TouchConversation :exec
UPDATE conversations
SET updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteConversation :exec
DELETE FROM conversations
WHERE id = ?;
