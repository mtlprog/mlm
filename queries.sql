-- name: GetState :one
SELECT * FROM states
WHERE (@user_id::bigint = 0::bigint OR user_id = @user_id)
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateState :exec
INSERT INTO states (user_id, state, data, meta, created_at)
  VALUES (@user_id, @state, @data, @meta, now());

-- name: GetReports :many
SELECT * FROM reports
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT nullif(@query_limit::int, 0);

-- name: GetReport :one
SELECT * FROM reports
WHERE deleted_at IS NULL AND
  id = @id;

-- name: GetPendingReport :one
SELECT * FROM reports
WHERE deleted_at IS NULL
  AND hash IS NULL
  AND created_at > now() - interval '24 hours'
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateReport :one
INSERT INTO reports (created_at, xdr)
  VALUES (now(), @xdr) RETURNING id;

-- name: DeleteReport :exec
UPDATE reports
SET deleted_at = now()
WHERE id = @id;

-- name: SetReportHash :exec
UPDATE reports
SET hash = @hash,
  updated_at = now()
WHERE id = @report_id;

-- name: GetReportRecommends :many
SELECT * FROM report_recommends
WHERE report_id = @report_id;

-- name: CreateReportRecommend :exec
INSERT INTO report_recommends (report_id, recommender, recommended, recommended_mtlap)
  VALUES (@report_id, @recommender, @recommended, @recommended_mtlap);

-- name: GetReportDistributes :many
SELECT * FROM report_distributes
WHERE report_id = @report_id;

-- name: CreateReportDistribute :exec
INSERT INTO report_distributes (report_id, recommender, asset, amount)
  VALUES (@report_id, @recommender, @asset, @amount);

-- name: GetReportConflicts :many
SELECT * FROM report_conflicts
WHERE report_id = @report_id;

-- name: CreateReportConflict :exec
INSERT INTO report_conflicts (report_id, recommender, recommended)
  VALUES (@report_id, @recommender, @recommended);

-- name: LockReport :exec
SELECT pg_advisory_lock(1);

-- name: UnlockReport :exec
SELECT pg_advisory_unlock(1);
