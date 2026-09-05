CREATE TABLE IF NOT EXISTS historical_imports (
    id BIGSERIAL PRIMARY KEY,
    reference_year SMALLINT NOT NULL CHECK (reference_year >= 1986),
    file_name VARCHAR(255) NOT NULL,
    file_sha256 CHAR(64) NOT NULL UNIQUE,
    file_size BIGINT NOT NULL CHECK (file_size > 0),
    source_url TEXT NOT NULL,
    parser_version VARCHAR(30) NOT NULL,
    layout_version VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'processing' CHECK (status IN ('processing', 'published', 'superseded', 'failed')),
    total_records BIGINT NOT NULL DEFAULT 0 CHECK (total_records >= 0),
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    superseded_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS historical_quotes (
    id BIGSERIAL PRIMARY KEY,
    import_id BIGINT NOT NULL REFERENCES historical_imports(id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL CHECK (line_number > 0),
    record_sha256 CHAR(64) NOT NULL,
    trading_date DATE NOT NULL,
    bdi_code VARCHAR(2) NOT NULL,
    ticker VARCHAR(12) NOT NULL,
    market_type SMALLINT NOT NULL,
    short_name VARCHAR(12) NOT NULL,
    specification VARCHAR(10) NOT NULL,
    term VARCHAR(3),
    currency VARCHAR(3) NOT NULL CHECK (currency = 'BRL'),
    open_price NUMERIC(20, 2) NOT NULL CHECK (open_price >= 0),
    high_price NUMERIC(20, 2) NOT NULL CHECK (high_price >= 0),
    low_price NUMERIC(20, 2) NOT NULL CHECK (low_price >= 0),
    average_price NUMERIC(20, 2) NOT NULL CHECK (average_price >= 0),
    close_price NUMERIC(20, 2) NOT NULL CHECK (close_price >= 0),
    best_bid_price NUMERIC(20, 2) NOT NULL CHECK (best_bid_price >= 0),
    best_ask_price NUMERIC(20, 2) NOT NULL CHECK (best_ask_price >= 0),
    trade_count INTEGER NOT NULL CHECK (trade_count >= 0),
    traded_quantity BIGINT NOT NULL CHECK (traded_quantity >= 0),
    traded_volume NUMERIC(20, 2) NOT NULL CHECK (traded_volume >= 0),
    strike_price NUMERIC(20, 2) NOT NULL DEFAULT 0 CHECK (strike_price >= 0),
    option_indicator VARCHAR(1),
    expiration_date DATE,
    quote_factor INTEGER NOT NULL CHECK (quote_factor > 0),
    strike_points NUMERIC(20, 6) NOT NULL DEFAULT 0,
    isin VARCHAR(12) NOT NULL CHECK (char_length(isin) = 12),
    distribution_number INTEGER NOT NULL CHECK (distribution_number >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_historical_quotes_import_line UNIQUE (import_id, line_number)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_historical_imports_published_year
    ON historical_imports (reference_year)
    WHERE status = 'published';

CREATE UNIQUE INDEX IF NOT EXISTS uq_historical_imports_processing_year
    ON historical_imports (reference_year)
    WHERE status = 'processing';

CREATE UNIQUE INDEX IF NOT EXISTS uq_historical_quotes_import_record
    ON historical_quotes (import_id, record_sha256);

CREATE INDEX IF NOT EXISTS idx_historical_quotes_ticker_date
    ON historical_quotes (ticker, trading_date DESC);

CREATE INDEX IF NOT EXISTS idx_historical_quotes_trading_date
    ON historical_quotes (trading_date);

CREATE INDEX IF NOT EXISTS idx_historical_quotes_isin_date
    ON historical_quotes (isin, trading_date DESC);

CREATE OR REPLACE VIEW published_historical_quotes AS
SELECT hq.*
FROM historical_quotes hq
JOIN historical_imports hi ON hi.id = hq.import_id
WHERE hi.status = 'published';
