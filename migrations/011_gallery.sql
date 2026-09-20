-- Estado permanente da FUT Gallery. Diferente de snapshots, não é podado:
-- itens vendidos continuam elegíveis e conclusões S continuam finais.
CREATE TABLE IF NOT EXISTS gallery_records (
    cycle text NOT NULL, club text NOT NULL, platform text NOT NULL,
    set_id text NOT NULL, payload jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cycle, club, platform, set_id)
);
CREATE TABLE IF NOT EXISTS gallery_cards (
    cycle text NOT NULL, club text NOT NULL, platform text NOT NULL,
    card_id bigint NOT NULL, payload jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cycle, club, platform, card_id)
);
CREATE TABLE IF NOT EXISTS gallery_overrides (
    cycle text NOT NULL, club text NOT NULL, platform text NOT NULL,
    card_id bigint NOT NULL, payload jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cycle, club, platform, card_id)
);
CREATE TABLE IF NOT EXISTS gallery_completions (
    cycle text NOT NULL, club text NOT NULL, platform text NOT NULL,
    set_id text NOT NULL, payload jsonb NOT NULL, completed_at timestamptz NOT NULL,
    PRIMARY KEY (cycle, club, platform, set_id)
);
