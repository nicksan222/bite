-- +goose Up
CREATE TABLE preferences (
    key        TEXT     PRIMARY KEY,
    value      TEXT     NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS preferences;
