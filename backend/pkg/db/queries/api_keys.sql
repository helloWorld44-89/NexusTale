-- name: UpsertAPIKey :one
INSERT INTO user_api_keys (user_id, provider, encrypted_key, key_hint)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, provider) DO UPDATE
    SET encrypted_key = EXCLUDED.encrypted_key,
        key_hint      = EXCLUDED.key_hint,
        updated_at    = now()
RETURNING id, user_id, provider, encrypted_key, key_hint, created_at, updated_at;

-- name: ListAPIKeys :many
SELECT id, user_id, provider, encrypted_key, key_hint, created_at, updated_at
FROM user_api_keys
WHERE user_id = $1
ORDER BY provider ASC;

-- name: GetAPIKey :one
SELECT id, user_id, provider, encrypted_key, key_hint, created_at, updated_at
FROM user_api_keys
WHERE user_id = $1 AND provider = $2;

-- name: DeleteAPIKey :exec
DELETE FROM user_api_keys
WHERE user_id = $1 AND provider = $2;
