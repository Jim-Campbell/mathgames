CREATE TABLE skill_state (
    skill          TEXT PRIMARY KEY,
    level          INT NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 10),
    xp             BIGINT NOT NULL DEFAULT 0,     -- lifetime XP earned in this skill
    streak         INT NOT NULL DEFAULT 0,        -- current consecutive-correct streak
    wrong_run      INT NOT NULL DEFAULT 0,        -- current consecutive-wrong run (zenkai)
    window_total   INT NOT NULL DEFAULT 0,        -- attempts in current adaptive window
    window_correct INT NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE questions (
    id           BIGSERIAL PRIMARY KEY,
    skill        TEXT NOT NULL,
    difficulty   INT NOT NULL CHECK (difficulty BETWEEN 1 AND 10),
    source       TEXT NOT NULL CHECK (source IN ('template','ai')),
    payload      JSONB NOT NULL,   -- prompt, kind, choices, display hints — NO answer
    answer       JSONB NOT NULL,   -- canonical answer (never serialized to the client pre-attempt)
    explanation  TEXT NOT NULL DEFAULT '',
    ai_model     TEXT,
    ai_batch_id  BIGINT,           -- REFERENCES ai_batches(id), added below
    times_served INT NOT NULL DEFAULT 0,
    retired      BOOLEAN NOT NULL DEFAULT FALSE,  -- parent can retire bad AI questions
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX questions_pick_idx ON questions(skill, difficulty, source)
    WHERE NOT retired;

CREATE TABLE sessions (
    id         BIGSERIAL PRIMARY KEY,
    mode       TEXT NOT NULL CHECK (mode IN ('training','quest','daily')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at   TIMESTAMPTZ
);

CREATE TABLE attempts (
    id           BIGSERIAL PRIMARY KEY,
    session_id   BIGINT REFERENCES sessions(id),
    question_id  BIGINT NOT NULL REFERENCES questions(id),
    skill        TEXT NOT NULL,
    difficulty   INT NOT NULL,           -- difficulty at time of attempt
    given        JSONB NOT NULL,         -- exactly what he answered
    correct      BOOLEAN NOT NULL,
    elapsed_ms   INT NOT NULL,
    xp_earned    INT NOT NULL,
    streak_after INT NOT NULL,
    level_after  INT NOT NULL,           -- skill level after adaptive update
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX attempts_skill_idx ON attempts(skill, created_at);
CREATE INDEX attempts_session_idx ON attempts(session_id);

CREATE TABLE unlocks (
    id         BIGSERIAL PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('fighter','dragon_ball','badge')),
    ref        TEXT NOT NULL,            -- fighter slug, ball number '1'..'7', badge slug
    source     TEXT NOT NULL DEFAULT '', -- human-readable: 'power_level 9000', 'saga saiyan ch3', 'wish'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, ref)
);

CREATE TABLE quest_chapters (
    id           BIGSERIAL PRIMARY KEY,
    saga         TEXT NOT NULL,          -- 'saiyan','namek','android','cell','buu' (in order)
    chapter      INT NOT NULL,           -- 1..N within the saga
    title        TEXT NOT NULL,
    story        TEXT NOT NULL,          -- AI-generated narrative shown on open
    requirement  JSONB NOT NULL,         -- {"correct": 12, "skills": ["multiplication","fractions"], "min_difficulty": 3}
    reward       JSONB NOT NULL,         -- {"xp": 500, "fighter": "vegeta", "dragon_ball": 2}  (any subset)
    progress     INT NOT NULL DEFAULT 0, -- correct answers counted toward requirement
    completed_at TIMESTAMPTZ,
    ai_batch_id  BIGINT,
    UNIQUE (saga, chapter)
);

CREATE TABLE daily_results (
    day          DATE PRIMARY KEY,
    question_ids BIGINT[] NOT NULL,      -- the 5 questions chosen for that day
    answered     INT NOT NULL DEFAULT 0,
    correct      INT NOT NULL DEFAULT 0,
    elapsed_ms   INT NOT NULL DEFAULT 0,
    xp_earned    INT NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ
);

CREATE TABLE ai_batches (
    id         BIGSERIAL PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('word_problems','logic','story')),
    skill      TEXT,
    difficulty INT,
    model      TEXT NOT NULL,
    prompt     TEXT NOT NULL,            -- the full prompt sent (versioned by content)
    raw        JSONB NOT NULL,           -- full raw API response for later analysis
    accepted   INT NOT NULL DEFAULT 0,   -- rows that passed validation and were inserted
    rejected   INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE questions      ADD CONSTRAINT questions_batch_fk      FOREIGN KEY (ai_batch_id) REFERENCES ai_batches(id);
ALTER TABLE quest_chapters ADD CONSTRAINT quest_chapters_batch_fk FOREIGN KEY (ai_batch_id) REFERENCES ai_batches(id);

CREATE TABLE settings (                   -- single row, id = 1
    id             INT PRIMARY KEY CHECK (id = 1),
    daily_count    INT NOT NULL DEFAULT 5,
    level_override JSONB NOT NULL DEFAULT '{}',  -- {"multiplication": 6} parent pin/boost; empty = adaptive
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO settings (id) VALUES (1) ON CONFLICT DO NOTHING;
