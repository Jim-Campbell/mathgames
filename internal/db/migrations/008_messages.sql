CREATE TABLE messages (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT NOT NULL DEFAULT 'message'
                CHECK (kind IN ('bug','idea','message')),
    body        TEXT NOT NULL,
    context     JSONB,                 -- {version, route, user_agent} auto-attached by the PWA
    emailed     BOOLEAN NOT NULL DEFAULT FALSE,
    email_error TEXT,                  -- last send error, if any
    read_at     TIMESTAMPTZ,           -- parent marked read
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX messages_created_idx ON messages (created_at DESC);
