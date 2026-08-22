-- Snapshot diário: o resultado completo de uma coleta+análise, gravado para
-- a API e a UI lerem sem recalcular.
--   psql "$EAFC_DSN" -f migrations/002_snapshots.sql

CREATE TABLE IF NOT EXISTS snapshots (
    id           BIGSERIAL   PRIMARY KEY,
    cycle        TEXT        NOT NULL,
    day          DATE        NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    squad_score  REAL        NOT NULL DEFAULT 0,
    coins        INTEGER     NOT NULL DEFAULT 0,
    payload      JSONB       NOT NULL,
    UNIQUE (cycle, day)
);

CREATE INDEX IF NOT EXISTS snapshots_recent_idx ON snapshots (cycle, day DESC);
