CREATE TABLE historical_imports (
    id BIGSERIAL PRIMARY KEY,
    reference_year SMALLINT NOT NULL CHECK (reference_year >= 1986),
    file_name VARCHAR(255) NOT NULL,
    file_sha256 CHAR(64) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'processing' CHECK (status IN ('processing', 'completed', 'failed')),
    total_records BIGINT NOT NULL DEFAULT 0 CHECK (total_records >= 0),
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE historical_quotes (
    id BIGSERIAL PRIMARY KEY,
    import_id BIGINT NOT NULL REFERENCES historical_imports(id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL CHECK (line_number > 0),
    trading_date DATE NOT NULL,
    bdi_code CHAR(2) NOT NULL,
    ticker VARCHAR(12) NOT NULL,
    market_type SMALLINT NOT NULL,
    short_name VARCHAR(12) NOT NULL,
    specification VARCHAR(10) NOT NULL,
    term VARCHAR(3),
    currency CHAR(4) NOT NULL,
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
    option_indicator CHAR(1),
    expiration_date DATE,
    quote_factor INTEGER NOT NULL CHECK (quote_factor > 0),
    strike_points NUMERIC(20, 6) NOT NULL DEFAULT 0,
    isin CHAR(12) NOT NULL,
    distribution_number INTEGER NOT NULL CHECK (distribution_number >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_historical_quotes_import_line UNIQUE (import_id, line_number)
);

CREATE INDEX idx_historical_quotes_ticker_date
    ON historical_quotes (ticker, trading_date DESC);

CREATE INDEX idx_historical_quotes_trading_date
    ON historical_quotes (trading_date);

CREATE INDEX idx_historical_quotes_isin_date
    ON historical_quotes (isin, trading_date DESC);
