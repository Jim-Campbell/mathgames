CREATE TABLE screen_time_resets (
    id               BIGSERIAL PRIMARY KEY,
    reset_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    minutes_redeemed INT NOT NULL,
    corrects_counted INT NOT NULL
);
ALTER TABLE settings ADD COLUMN minutes_per_correct INT NOT NULL DEFAULT 3;
