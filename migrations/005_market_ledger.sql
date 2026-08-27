-- Estado local do plano de mercado. Não contém credenciais, ordens ou dados
-- de conta EA: watchlist é editável; ledger é append-only por ciclo.
--   psql "$EAFC_DSN" -f migrations/005_market_ledger.sql

CREATE TABLE IF NOT EXISTS market_watchlist (
    cycle      TEXT        NOT NULL,
    id         TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload    JSONB       NOT NULL,
    PRIMARY KEY (cycle, id)
);

CREATE TABLE IF NOT EXISTS market_ledger (
    cycle       TEXT        NOT NULL,
    id          TEXT        NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload     JSONB       NOT NULL,
    PRIMARY KEY (cycle, id)
);

CREATE INDEX IF NOT EXISTS market_ledger_recent_idx ON market_ledger (cycle, recorded_at DESC);
