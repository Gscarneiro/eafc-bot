-- Série temporal do custo da solução mais barata de cada desafio de SBC —
-- o fut.gg já resolve o fodder mais barato que bate o requisito; rastrear
-- esse número ao longo do tempo é o sinal de demanda esquentando (subindo)
-- ou esfriando, sem reimplementar um solver de squad com química.
--   psql "$EAFC_DSN" -f migrations/003_sbc_cost.sql

CREATE TABLE IF NOT EXISTS sbc_cost_ticks (
    challenge_key TEXT        NOT NULL, -- ver store.SBCChallengeKey
    cycle         TEXT        NOT NULL,
    observed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    coins         INTEGER     NOT NULL,
    PRIMARY KEY (challenge_key, cycle, observed_at)
);

CREATE INDEX IF NOT EXISTS sbc_cost_ticks_lookup_idx ON sbc_cost_ticks (challenge_key, cycle, observed_at DESC);
