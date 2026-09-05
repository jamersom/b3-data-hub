-- name: GetHistoricalImportForUpdate :one
SELECT id, status, total_records
FROM historical_imports
WHERE file_sha256 = $1
FOR UPDATE;

-- name: CreateHistoricalImport :one
INSERT INTO historical_imports (
    reference_year, file_name, file_sha256, file_size,
    source_url, parser_version, layout_version, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'processing')
RETURNING id;

-- name: DeleteHistoricalQuotesByImportID :exec
DELETE FROM historical_quotes
WHERE import_id = $1;

-- name: RestartHistoricalImport :exec
UPDATE historical_imports
SET status = 'processing',
    total_records = 0,
    error_message = NULL,
    started_at = NOW(),
    completed_at = NULL,
    published_at = NULL,
    superseded_at = NULL
WHERE id = $1;

-- name: FailHistoricalImport :exec
UPDATE historical_imports
SET status = 'failed',
    error_message = $2,
    completed_at = NOW()
WHERE id = $1;
