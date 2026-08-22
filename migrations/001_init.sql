-- Schema do eafc-bot. Roda em qualquer Postgres 13+.
--   psql "$EAFC_DSN" -f migrations/001_init.sql

CREATE TABLE IF NOT EXISTS players (
    ea_id        BIGINT       NOT NULL,
    cycle        TEXT         NOT NULL,
    name         TEXT         NOT NULL,
    common_name  TEXT,
    rating       SMALLINT     NOT NULL,
    position     TEXT         NOT NULL,
    alt_positions TEXT[]      NOT NULL DEFAULT '{}',
    version      TEXT,
    club         TEXT,
    league       TEXT,
    nation       TEXT,
    pace         SMALLINT, shooting SMALLINT, passing SMALLINT,
    dribbling    SMALLINT, defending SMALLINT, physical SMALLINT,
    play_styles  JSONB        NOT NULL DEFAULT '[]',
    weak_foot    SMALLINT, skill_moves SMALLINT,
    first_seen   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    last_seen    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (ea_id, cycle)
);

CREATE INDEX IF NOT EXISTS players_position_rating_idx ON players (cycle, position, rating DESC);
CREATE INDEX IF NOT EXISTS players_first_seen_idx      ON players (cycle, first_seen DESC);

-- Série temporal de preços: é o que responde "subiu ou caiu desde ontem?"
-- e alimenta a seção de mercado do relatório.
CREATE TABLE IF NOT EXISTS price_ticks (
    ea_id      BIGINT      NOT NULL,
    cycle      TEXT        NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    coins      INTEGER     NOT NULL,
    extinct    BOOLEAN     NOT NULL DEFAULT false,
    PRIMARY KEY (ea_id, cycle, observed_at)
);

CREATE INDEX IF NOT EXISTS price_ticks_lookup_idx ON price_ticks (ea_id, cycle, observed_at DESC);

-- Retrato diário do clube, para saber o que mudou no elenco entre execuções.
CREATE TABLE IF NOT EXISTS club_snapshots (
    id         BIGSERIAL   PRIMARY KEY,
    gamer_tag  TEXT        NOT NULL,
    cycle      TEXT        NOT NULL,
    synced_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    coins      INTEGER     NOT NULL DEFAULT 0,
    payload    JSONB       NOT NULL
);

CREATE INDEX IF NOT EXISTS club_snapshots_tag_idx ON club_snapshots (gamer_tag, cycle, synced_at DESC);

-- Notícias já vistas, para o relatório diário só mostrar o que é novo.
CREATE TABLE IF NOT EXISTS seen_news (
    id           TEXT        PRIMARY KEY,
    cycle        TEXT        NOT NULL,
    title        TEXT        NOT NULL,
    url          TEXT,
    published_at TIMESTAMPTZ,
    first_seen   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Recomendações emitidas, para o bot não repetir todo dia a mesma sugestão
-- e para dá depois medir se o conselho foi bom.
CREATE TABLE IF NOT EXISTS recommendations (
    id          BIGSERIAL   PRIMARY KEY,
    cycle       TEXT        NOT NULL,
    kind        TEXT        NOT NULL,   -- 'upgrade' | 'evolution' | 'sbc' | 'objective'
    slot        TEXT,
    subject_id  BIGINT,                 -- ea_id da carta alvo, quando aplicável
    gain        REAL,
    net_cost    INTEGER,
    payload     JSONB       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS recommendations_recent_idx ON recommendations (cycle, kind, created_at DESC);
