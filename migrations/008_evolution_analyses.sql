CREATE TABLE IF NOT EXISTS evolution_analyses (
    id TEXT PRIMARY KEY,
    cycle TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS evolution_analyses_cycle_hash_idx
    ON evolution_analyses (cycle, input_hash, updated_at DESC);
