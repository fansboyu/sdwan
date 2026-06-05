-- name: CreateEmailVerification :one
INSERT INTO email_verifications (id, email, purpose, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, purpose, code_hash, expires_at, consumed_at, attempt_count, created_at;

-- name: GetLatestEmailVerification :one
SELECT id, email, purpose, code_hash, expires_at, consumed_at, attempt_count, created_at
FROM email_verifications
WHERE email = $1 AND purpose = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ConsumeEmailVerification :exec
UPDATE email_verifications
SET consumed_at = now()
WHERE id = $1 AND consumed_at IS NULL;

-- name: IncrementEmailVerificationAttempts :exec
UPDATE email_verifications
SET attempt_count = attempt_count + 1
WHERE id = $1;
