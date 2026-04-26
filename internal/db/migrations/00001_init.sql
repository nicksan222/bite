-- +goose Up
CREATE TABLE conversations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT     NOT NULL DEFAULT '',
    model      TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER  NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT     NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content         TEXT     NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_conversation
    ON messages (conversation_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_messages_conversation;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
