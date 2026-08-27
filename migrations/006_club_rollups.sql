-- Memória compacta da coleção, independente da retenção de snapshots grandes.
--   psql "$EAFC_DSN" -f migrations/006_club_rollups.sql

CREATE TABLE IF NOT EXISTS club_rollups (
    cycle       TEXT        NOT NULL,
    day         DATE        NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    payload     JSONB       NOT NULL,
    PRIMARY KEY (cycle, day)
);

CREATE INDEX IF NOT EXISTS club_rollups_recent_idx ON club_rollups (cycle, day DESC);
