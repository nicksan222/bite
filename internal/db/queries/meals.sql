-- name: SaveMeal :one
INSERT INTO meals (title, description, items, kcal, protein_g, carbs_g, fat_g, eaten_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMeal :one
SELECT * FROM meals WHERE id = ? LIMIT 1;

-- name: ListMealsBetween :many
SELECT * FROM meals
WHERE eaten_at >= ? AND eaten_at < ?
ORDER BY eaten_at ASC;

-- name: ListRecentMeals :many
SELECT * FROM meals
ORDER BY eaten_at DESC
LIMIT ?;

-- name: DeleteMeal :exec
DELETE FROM meals WHERE id = ?;
