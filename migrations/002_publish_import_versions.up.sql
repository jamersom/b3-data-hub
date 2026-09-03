BEGIN;

DROP VIEW IF EXISTS published_historical_quotes;

ALTER TABLE historical_imports
    DROP CONSTRAINT IF EXISTS historical_imports_status_check;

ALTER TABLE historical_imports
    ADD COLUMN IF NOT EXISTS file_size BIGINT,
    ADD COLUMN IF NOT EXISTS source_url TEXT,
    ADD COLUMN IF NOT EXISTS parser_version VARCHAR(30),
    ADD COLUMN IF NOT EXISTS layout_version VARCHAR(30),
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS superseded_at TIMESTAMPTZ;

UPDATE historical_imports
SET file_size = COALESCE(file_size, 1),
    source_url = COALESCE(source_url, ''),
    parser_version = COALESCE(parser_version, 'legacy'),
    layout_version = COALESCE(layout_version, 'COTAHIST-2017-01');

ALTER TABLE historical_imports
    ALTER COLUMN file_size SET NOT NULL,
    ALTER COLUMN source_url SET NOT NULL,
    ALTER COLUMN parser_version SET NOT NULL,
    ALTER COLUMN layout_version SET NOT NULL;

ALTER TABLE historical_imports
    DROP CONSTRAINT IF EXISTS historical_imports_file_size_check,
    ADD CONSTRAINT historical_imports_file_size_check CHECK (file_size > 0);

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY reference_year
               ORDER BY completed_at DESC NULLS LAST, id DESC
           ) AS position
    FROM historical_imports
    WHERE status = 'completed'
)
UPDATE historical_imports hi
SET status = CASE WHEN ranked.position = 1 THEN 'published' ELSE 'superseded' END,
    published_at = CASE WHEN ranked.position = 1 THEN COALESCE(hi.completed_at, NOW()) ELSE hi.published_at END,
    superseded_at = CASE WHEN ranked.position > 1 THEN NOW() ELSE hi.superseded_at END
FROM ranked
WHERE hi.id = ranked.id;

WITH ranked_processing AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY reference_year
               ORDER BY started_at DESC, id DESC
           ) AS position
    FROM historical_imports
    WHERE status = 'processing'
)
UPDATE historical_imports hi
SET status = 'failed',
    error_message = 'superseded incomplete import during publication migration',
    completed_at = NOW()
FROM ranked_processing
WHERE hi.id = ranked_processing.id
  AND ranked_processing.position > 1;

DELETE FROM historical_quotes hq
USING historical_imports hi
WHERE hq.import_id = hi.id
  AND hi.status IN ('superseded', 'failed');

ALTER TABLE historical_imports
    ADD CONSTRAINT historical_imports_status_check
    CHECK (status IN ('processing', 'published', 'superseded', 'failed'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_historical_imports_published_year
    ON historical_imports (reference_year)
    WHERE status = 'published';

CREATE UNIQUE INDEX IF NOT EXISTS uq_historical_imports_processing_year
    ON historical_imports (reference_year)
    WHERE status = 'processing';

ALTER TABLE historical_quotes
    ADD COLUMN IF NOT EXISTS record_sha256 CHAR(64);

UPDATE historical_quotes
SET record_sha256 = md5(
        concat_ws('|', trading_date, bdi_code, ticker, market_type, short_name,
                  specification, term, currency, open_price, high_price, low_price,
                  average_price, close_price, best_bid_price, best_ask_price,
                  trade_count, traded_quantity, traded_volume, strike_price,
                  option_indicator, expiration_date, quote_factor, strike_points,
                  isin, distribution_number)
    ) || md5(
        concat_ws('|', distribution_number, isin, strike_points, quote_factor,
                  expiration_date, option_indicator, strike_price, traded_volume,
                  traded_quantity, trade_count, best_ask_price, best_bid_price,
                  close_price, average_price, low_price, high_price, open_price,
                  currency, term, specification, short_name, market_type, ticker,
                  bdi_code, trading_date)
    )
WHERE record_sha256 IS NULL;

ALTER TABLE historical_quotes
    ALTER COLUMN record_sha256 SET NOT NULL,
    ALTER COLUMN bdi_code TYPE VARCHAR(2) USING btrim(bdi_code),
    ALTER COLUMN option_indicator TYPE VARCHAR(1) USING NULLIF(btrim(option_indicator), ''),
    ALTER COLUMN isin TYPE VARCHAR(12) USING btrim(isin),
    ALTER COLUMN currency TYPE VARCHAR(3)
        USING CASE btrim(currency) WHEN 'R$' THEN 'BRL' ELSE btrim(currency) END;

ALTER TABLE historical_quotes
    DROP CONSTRAINT IF EXISTS historical_quotes_currency_check,
    DROP CONSTRAINT IF EXISTS historical_quotes_isin_check;

ALTER TABLE historical_quotes
    ADD CONSTRAINT historical_quotes_currency_check CHECK (currency = 'BRL'),
    ADD CONSTRAINT historical_quotes_isin_check CHECK (char_length(isin) = 12);

CREATE UNIQUE INDEX IF NOT EXISTS uq_historical_quotes_import_record
    ON historical_quotes (import_id, record_sha256);

CREATE VIEW published_historical_quotes AS
SELECT hq.*
FROM historical_quotes hq
JOIN historical_imports hi ON hi.id = hq.import_id
WHERE hi.status = 'published';

COMMIT;
