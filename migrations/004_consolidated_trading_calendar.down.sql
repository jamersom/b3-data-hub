BEGIN;


-- Aggregate all tickers, not one asset. Unpublished imports never contribute.
CREATE OR REPLACE VIEW observed_trading_sessions AS
SELECT i.id AS import_id, i.reference_year, q.market_type, q.trading_date,
       count(*) AS quote_records, count(DISTINCT q.ticker) AS ticker_count,
       'cotahist_observed'::text AS source
FROM historical_imports i
JOIN historical_quotes q ON q.import_id = i.id
WHERE i.status = 'published' AND q.trade_count > 0
  AND extract(year FROM q.trading_date) = i.reference_year
GROUP BY i.id, i.reference_year, q.market_type, q.trading_date;

-- File integrity is distinct from independent verification of exchange dates.
CREATE OR REPLACE VIEW observed_calendar_coverage AS
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
    FROM observed_trading_sessions
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

COMMENT ON VIEW observed_trading_sessions IS
'Dates with trades across all tickers in published COTAHIST imports. Absence is not proof of a holiday.';
COMMENT ON VIEW observed_calendar_coverage IS
'Observed bounds and import integrity by year/market. Integrity does not imply an independently verified exchange calendar.';



DROP TRIGGER IF EXISTS refresh_observed_calendar ON historical_imports;
DROP FUNCTION IF EXISTS refresh_observed_calendar_on_publication();
DROP TABLE IF EXISTS trading_calendar_coverage;
DROP TABLE IF EXISTS trading_session_calendar;
DROP VIEW IF EXISTS observed_calendar_coverage_source;
DROP VIEW IF EXISTS observed_trading_sessions_source;
COMMIT;