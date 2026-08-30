-- Caminhos de evoluÃ§Ã£o escolhidos manualmente. Ã‰ estado local do bot;
-- nunca Ã© enviado Ã  EA nem representa uma evoluÃ§Ã£o aplicada no jogo.
-- Rode apÃ³s as migraÃ§Ãµes anteriores:
--   psql "$EAFC_DSN" -f migrations/009_saved_evolution_paths.sql

CREATE TABLE IF NOT EXISTS saved_evolution_paths (
    cycle    TEXT        NOT NULL,
    id       TEXT        NOT NULL,
    saved_at TIMESTAMPTZ NOT NULL,
    payload  JSONB       NOT NULL,
    PRIMARY KEY (cycle, id)
);

CREATE INDEX IF NOT EXISTS saved_evolution_paths_recent_idx
    ON saved_evolution_paths (cycle, saved_at DESC);
