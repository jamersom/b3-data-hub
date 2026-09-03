BEGIN;

DROP VIEW IF EXISTS published_historical_quotes;
DROP INDEX IF EXISTS uq_historical_quotes_import_record;
DROP INDEX IF EXISTS uq_historical_imports_published_year;
DROP INDEX IF EXISTS uq_historical_imports_processing_year;

ALTER TABLE historical_quotes
    DROP CONSTRAINT IF EXISTS historical_quotes_currency_check,
    DROP CONSTRAINT IF EXISTS historical_quotes_isin_check,
    DROP COLUMN IF EXISTS record_sha256,
    ALTER COLUMN bdi_code TYPE CHAR(2),
    ALTER COLUMN option_indicator TYPE CHAR(1),
    ALTER COLUMN isin TYPE CHAR(12),
    ALTER COLUMN currency TYPE CHAR(4);

UPDATE historical_imports
SET status = CASE WHEN status = 'published' THEN 'completed' ELSE status END;

DELETE FROM historical_imports WHERE status = 'superseded';

ALTER TABLE historical_imports
    DROP CONSTRAINT IF EXISTS historical_imports_status_check,
    DROP CONSTRAINT IF EXISTS historical_imports_file_size_check,
    DROP COLUMN IF EXISTS file_size,
    DROP COLUMN IF EXISTS source_url,
    DROP COLUMN IF EXISTS parser_version,
    DROP COLUMN IF EXISTS layout_version,
    DROP COLUMN IF EXISTS published_at,
    DROP COLUMN IF EXISTS superseded_at,
    ADD CONSTRAINT historical_imports_status_check
        CHECK (status IN ('processing', 'completed', 'failed'));

COMMIT;
