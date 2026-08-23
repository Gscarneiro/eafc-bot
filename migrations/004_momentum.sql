-- Cache do momentum mais recente lido do fut.gg — não é série temporal (o
-- fut.gg já é a série; ver domain.Player.MomentumPct), só o último valor,
-- pra internal/api montar a tela de investimentos sem nunca tocar a rede.
--   psql "$EAFC_DSN" -f migrations/004_momentum.sql

CREATE TABLE IF NOT EXISTS momentum_cache (
    cycle      TEXT        PRIMARY KEY,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload    JSONB       NOT NULL
);
