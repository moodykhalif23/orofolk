-- Custom report builder (Pack 3 §1). Definitions are org-scoped; runs/schedules
-- are authorized via their parent definition (handler fetches the def first).

-- ===== Definitions =========================================================

-- name: CreateReportDefinition :one
INSERT INTO report_definitions (organization_id, name, entity, dimensions, measures, filters, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListReportDefinitions :many
SELECT * FROM report_definitions WHERE organization_id = $1 ORDER BY name;

-- name: GetReportDefinition :one
SELECT * FROM report_definitions WHERE organization_id = $1 AND id = $2;

-- name: UpdateReportDefinition :one
UPDATE report_definitions
   SET name = $3, entity = $4, dimensions = $5, measures = $6, filters = $7
 WHERE organization_id = $1 AND id = $2
RETURNING *;

-- name: DeleteReportDefinition :exec
DELETE FROM report_definitions WHERE organization_id = $1 AND id = $2;

-- ===== Runs ================================================================

-- name: CreateReportRun :one
INSERT INTO report_runs (report_definition_id, trigger, status)
VALUES ($1, $2, 'running')
RETURNING id;

-- name: FinishReportRun :one
UPDATE report_runs
   SET status = $2, row_count = $3, file_name = $4, content_type = $5,
       file_bytes = $6, file_url = $7, error = $8, finished_at = $9
 WHERE id = $1
RETURNING id, report_definition_id, status, trigger, row_count, file_name, file_url, error, started_at, finished_at;

-- name: ListReportRuns :many
SELECT id, report_definition_id, status, trigger, row_count, file_name, file_url, error, started_at, finished_at
FROM report_runs
WHERE report_definition_id = $1
ORDER BY started_at DESC
LIMIT 100;

-- name: GetReportRunArtifact :one
SELECT rr.id, rr.file_name, rr.content_type, rr.file_bytes
FROM report_runs rr
JOIN report_definitions d ON d.id = rr.report_definition_id
WHERE rr.id = $1 AND d.organization_id = $2;

-- ===== Schedules ===========================================================

-- name: CreateReportSchedule :one
INSERT INTO report_schedules (report_definition_id, cadence, format, recipients, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListReportSchedules :many
SELECT * FROM report_schedules WHERE report_definition_id = $1 ORDER BY id;

-- name: DeleteReportSchedule :exec
DELETE FROM report_schedules WHERE id = $1 AND report_definition_id = $2;

-- name: SetReportScheduleLastRun :exec
UPDATE report_schedules SET last_run_at = $2 WHERE id = $1;

-- ListDueReportSchedules returns active schedules whose cadence interval has
-- elapsed since last_run_at (or that have never run), joined to their definition
-- so the job can compile + run without a second query.
-- name: ListDueReportSchedules :many
SELECT s.id AS schedule_id, s.format, s.recipients, s.cadence,
       d.id AS definition_id, d.organization_id, d.name, d.entity,
       d.dimensions, d.measures, d.filters
FROM report_schedules s
JOIN report_definitions d ON d.id = s.report_definition_id
WHERE s.is_active AND (
      s.last_run_at IS NULL
   OR (s.cadence = 'daily'   AND s.last_run_at < now() - interval '1 day')
   OR (s.cadence = 'weekly'  AND s.last_run_at < now() - interval '7 days')
   OR (s.cadence = 'monthly' AND s.last_run_at < now() - interval '1 month'))
ORDER BY s.id;
