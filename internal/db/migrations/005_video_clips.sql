CREATE TABLE clips (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT NOT NULL,
    r2_key       TEXT NOT NULL,
    url          TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    duration_ms  INT,                    -- client-reported, nullable
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    on_correct   BOOLEAN NOT NULL DEFAULT TRUE,
    on_wrong     BOOLEAN NOT NULL DEFAULT FALSE,
    weight       INT NOT NULL DEFAULT 1 CHECK (weight > 0),
    play_count   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE clip_plays (
    id         BIGSERIAL PRIMARY KEY,
    clip_id    BIGINT NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    attempt_id BIGINT REFERENCES attempts(id),
    trigger    TEXT NOT NULL CHECK (trigger IN ('correct','wrong')),
    played_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE settings ADD COLUMN clip_chance      INT NOT NULL DEFAULT 40 CHECK (clip_chance > 0);
ALTER TABLE settings ADD COLUMN clip_session_cap INT NOT NULL DEFAULT 2  CHECK (clip_session_cap >= 0);
