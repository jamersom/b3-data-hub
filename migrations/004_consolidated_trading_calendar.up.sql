BEGIN;
-- Serialize the initial snapshot with imports and publication.
LOCK TABLE historical_imports, historical_quotes IN SHARE ROW EXCLUSIVE MODE;


-- Aggregate all tickers, not one asset. Unpublished imports never contribute.
CREATE OR REPLACE VIEW observed_trading_sessions_source AS
SELECT i.id AS import_id, i.reference_year, q.market_type, q.trading_date,
       count(*) AS quote_records, count(DISTINCT q.ticker) AS ticker_count,
       'cotahist_observed'::text AS source
FROM historical_imports i
JOIN historical_quotes q ON q.import_id = i.id
WHERE i.status = 'published' AND q.trade_count > 0
  AND extract(year FROM q.trading_date) = i.reference_year
GROUP BY i.id, i.reference_year, q.market_type, q.trading_date;

-- File integrity is distinct from independent verification of exchange dates.
CREATE OR REPLACE VIEW observed_calendar_coverage_source AS
WITH inventory AS (
    SELECT i.id AS import_id, count(q.id) AS actual_records,
           count(q.id) FILTER (WHERE extract(year FROM q.trading_date) <> i.reference_year) AS wrong_year_records
    FROM historical_imports i
    LEFT JOIN historical_quotes q ON q.import_id = i.id
    WHERE i.status = 'published'
    GROUP BY i.id
), sessions AS (
    SELECT import_id, market_type, min(trading_date) AS observed_from,
           max(trading_date) AS observed_to, count(*) AS session_count,
           md5(string_agg(to_char(trading_date, 'YYYY-MM-DD') || ':' || quote_records::text || ':' || ticker_count::text,
                          ',' ORDER BY trading_date)) AS sessions_digest
    FROM observed_trading_sessions_source
    GROUP BY import_id, market_type
)
SELECT i.id AS import_id, i.reference_year, s.market_type,
       s.observed_from, s.observed_to, coalesce(s.session_count, 0) AS session_count,
       i.total_records AS declared_records, v.actual_records,
       (i.total_records > 0 AND i.total_records = v.actual_records
        AND v.wrong_year_records = 0 AND s.session_count > 0) IS TRUE AS import_integrity_validated,
       false AS official_calendar_verified,
       'cotahist_observed'::text AS source,
       i.file_sha256, i.parser_version, i.layout_version, i.published_at,
       'observed-v1:' || md5(concat_ws('|', i.id::text, i.file_sha256,
           i.parser_version, i.layout_version,
           extract(epoch FROM i.published_at)::text, i.total_records::text,
           v.actual_records::text, v.wrong_year_records::text,
           s.market_type::text, s.sessions_digest)) AS calendar_version
FROM historical_imports i
JOIN inventory v ON v.import_id = i.id
LEFT JOIN sessions s ON s.import_id = i.id
WHERE i.status = 'published';

COMMENT ON VIEW observed_trading_sessions_source IS
'Dates with trades across all tickers in published COTAHIST imports. Absence is not proof of a holiday.';
COMMENT ON VIEW observed_calendar_coverage_source IS
'Observed bounds and import integrity by year/market. Integrity does not imply an independently verified exchange calendar.';



CREATE TABLE IF NOT EXISTS trading_session_calendar AS
SELECT * FROM observed_trading_sessions_source WITH NO DATA;
CREATE UNIQUE INDEX IF NOT EXISTS uq_trading_session_calendar
 ON trading_session_calendar(import_id, market_type, trading_date);
CREATE INDEX IF NOT EXISTS idx_trading_session_calendar_market_date
 ON trading_session_calendar(market_type, trading_date);

CREATE TABLE IF NOT EXISTS trading_calendar_coverage AS
SELECT * FROM observed_calendar_coverage_source WITH NO DATA;
CREATE UNIQUE INDEX IF NOT EXISTS uq_trading_calendar_coverage
 ON trading_calendar_coverage(import_id, market_type) NULLS NOT DISTINCT;
CREATE INDEX IF NOT EXISTS idx_trading_calendar_coverage_market_year
 ON trading_calendar_coverage(market_type, reference_year);

CREATE OR REPLACE FUNCTION refresh_observed_calendar_on_publication()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 DELETE FROM trading_session_calendar WHERE import_id = OLD.id;
 DELETE FROM trading_calendar_coverage WHERE import_id = OLD.id;
 IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
 IF NEW.status = 'published' THEN
   INSERT INTO trading_session_calendar
   SELECT * FROM observed_trading_sessions_source WHERE import_id = NEW.id;
   INSERT INTO trading_calendar_coverage
   SELECT * FROM observed_calendar_coverage_source WHERE import_id = NEW.id;
 END IF;
 RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS refresh_observed_calendar ON historical_imports;
CREATE TRIGGER refresh_observed_calendar
AFTER INSERT OR UPDATE OF status, total_records, file_sha256, parser_version, layout_version, published_at
OR DELETE ON historical_imports
FOR EACH ROW EXECUTE FUNCTION refresh_observed_calendar_on_publication();

-- Backfill existing publications. Only derived data is replaced.
DELETE FROM trading_session_calendar;
DELETE FROM trading_calendar_coverage;
INSERT INTO trading_session_calendar SELECT * FROM observed_trading_sessions_source;
INSERT INTO trading_calendar_coverage SELECT * FROM observed_calendar_coverage_source;

-- Preserve existing consumer SQL and column types, now reading small tables.
CREATE OR REPLACE VIEW observed_trading_sessions AS
SELECT * FROM trading_session_calendar;
CREATE OR REPLACE VIEW observed_calendar_coverage AS
SELECT * FROM trading_calendar_coverage;

ANALYZE trading_session_calendar;
ANALYZE trading_calendar_coverage;
COMMIT;
