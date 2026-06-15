-- Generic import engine (Platform roadmap, Phase 3): dry-run runs + per-row
-- verdicts, applied on commit.

-- name: CreateImportRun :one
INSERT INTO import_runs (organization_id, target, format, source_filename, total_rows, create_rows, update_rows, error_rows, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetImportRun :one
SELECT * FROM import_runs WHERE organization_id = $1 AND id = $2;

-- name: ListImportRuns :many
SELECT * FROM import_runs WHERE organization_id = $1 ORDER BY created_at DESC LIMIT 100;

-- name: MarkImportRunCommitted :exec
UPDATE import_runs SET status = 'committed', committed_at = now()
WHERE organization_id = $1 AND id = $2 AND status = 'validated';

-- name: CreateImportRow :exec
INSERT INTO import_rows (import_run_id, organization_id, row_number, data, status, message)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListImportRows :many
SELECT row_number, data, status, message FROM import_rows
WHERE import_run_id = $1
ORDER BY row_number
LIMIT $2 OFFSET $3;

-- ListCommittableImportRows returns the rows a commit will apply (create/update).
-- name: ListCommittableImportRows :many
SELECT row_number, data, status FROM import_rows
WHERE import_run_id = $1 AND status IN ('create', 'update')
ORDER BY row_number;
