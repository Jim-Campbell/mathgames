ALTER TABLE screen_time_resets
    ADD COLUMN reason TEXT NOT NULL DEFAULT 'manual'
        CHECK (reason IN ('manual','daily')),
    ADD COLUMN day DATE;

CREATE UNIQUE INDEX screen_time_daily_uniq
    ON screen_time_resets (day) WHERE reason = 'daily';
