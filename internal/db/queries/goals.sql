-- name: GetGoals :one
SELECT * FROM goals ORDER BY id LIMIT 1;

-- name: SetGoals :one
UPDATE goals SET
    kcal       = ?,
    protein_g  = ?,
    carbs_g    = ?,
    fat_g      = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = (SELECT id FROM goals ORDER BY id LIMIT 1)
RETURNING *;
