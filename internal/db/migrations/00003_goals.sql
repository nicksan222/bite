-- +goose Up
CREATE TABLE goals (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    kcal       REAL,             -- daily calorie target (NULL = no target)
    protein_g  REAL,             -- daily protein floor in grams (NULL = no target)
    carbs_g    REAL,             -- daily carb limit in grams (NULL = no target)
    fat_g      REAL,             -- daily fat limit in grams (NULL = no target)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed with a single sentinel row so queries always return something.
INSERT INTO goals (kcal, protein_g, carbs_g, fat_g) VALUES (NULL, NULL, NULL, NULL);

-- +goose Down
DROP TABLE IF EXISTS goals;
