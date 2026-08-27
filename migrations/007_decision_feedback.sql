CREATE TABLE IF NOT EXISTS decision_feedback (
    cycle TEXT NOT NULL,
    id TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    PRIMARY KEY (cycle, id)
);

CREATE INDEX IF NOT EXISTS decision_feedback_cycle_recorded_idx
    ON decision_feedback (cycle, recorded_at);
