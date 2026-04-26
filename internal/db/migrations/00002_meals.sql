-- +goose Up
CREATE TABLE meals (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    title       TEXT     NOT NULL,
    description TEXT     NOT NULL DEFAULT '',
    items       TEXT     NOT NULL DEFAULT '[]',  -- JSON array of strings
    kcal        REAL     NOT NULL DEFAULT 0,
    protein_g   REAL     NOT NULL DEFAULT 0,
    carbs_g     REAL     NOT NULL DEFAULT 0,
    fat_g       REAL     NOT NULL DEFAULT 0,
    eaten_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_meals_eaten_at ON meals (eaten_at);

-- +goose Down
DROP INDEX IF EXISTS idx_meals_eaten_at;
DROP TABLE IF EXISTS meals;
