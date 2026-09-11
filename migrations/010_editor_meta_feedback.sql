-- Estado local criado pela avaliação contextual e pelo editor de elenco.
-- As três tabelas são particionadas pelo ciclo; planos também pelo clube.

CREATE TABLE IF NOT EXISTS saved_squad_plans (
    cycle       TEXT        NOT NULL,
    club        TEXT        NOT NULL,
    id          TEXT        NOT NULL,
    is_reference BOOLEAN    NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL,
    payload     JSONB       NOT NULL,
    PRIMARY KEY (cycle, club, id)
);

CREATE INDEX IF NOT EXISTS saved_squad_plans_recent_idx
    ON saved_squad_plans (cycle, club, is_reference DESC, updated_at DESC);

CREATE TABLE IF NOT EXISTS gameplay_feedback (
    cycle          TEXT        NOT NULL,
    comparison_id  TEXT        NOT NULL,
    id              TEXT        NOT NULL,
    registered_at   TIMESTAMPTZ NOT NULL,
    payload         JSONB       NOT NULL,
    PRIMARY KEY (cycle, comparison_id)
);

CREATE INDEX IF NOT EXISTS gameplay_feedback_recent_idx
    ON gameplay_feedback (cycle, registered_at DESC);

CREATE TABLE IF NOT EXISTS meta_proposals (
    cycle       TEXT        NOT NULL,
    id          TEXT        NOT NULL,
    package_hash TEXT       NOT NULL,
    status      TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    payload     JSONB       NOT NULL,
    PRIMARY KEY (cycle, id)
);

CREATE INDEX IF NOT EXISTS meta_proposals_recent_idx
    ON meta_proposals (cycle, created_at DESC);
