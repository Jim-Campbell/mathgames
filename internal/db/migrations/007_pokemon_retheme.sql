-- Retheme: Dragon Ball Z -> Pokemon (build-prompts/retheme-pokemon.md).
-- Reskins names/slugs/kinds only; no mechanic/threshold changes except the
-- one deliberate bump: 7 Dragon Balls -> 8 Gym Badges.

-- ---- unlock kinds ----

ALTER TABLE unlocks DROP CONSTRAINT unlocks_kind_check;

-- Old fighter slugs (krillin, goku, vegeta, ...) don't exist in the new
-- Pokedex catalog; the app is barely used, so we wipe them and let the
-- Pokedex restart clean rather than trying to remap slug-for-slug.
DELETE FROM unlocks WHERE kind = 'fighter';
UPDATE unlocks SET kind = 'gym_badge' WHERE kind = 'dragon_ball';
UPDATE unlocks SET kind = 'ribbon'    WHERE kind = 'badge';

ALTER TABLE unlocks ADD CONSTRAINT unlocks_kind_check
    CHECK (kind IN ('pokemon','gym_badge','ribbon'));

-- ---- quests: re-seed 5 gym arcs x 4 chapters ----
--
-- 5 Pokemon gym/region arcs (pewter, cerulean, celadon, fuchsia, cinnabar),
-- in story order, replacing the 5 DBZ sagas (saiyan, namek, android, cell,
-- buu) 1:1 -- same skill/difficulty/XP progression per chapter, only the
-- saga key, title, and reward vocabulary change. Titles/story are
-- placeholders; AI batches (kind='story', phase 4) rewrite title/story per
-- arc. The 8 gym badges are scattered across chapters (was 7) so the Master
-- Ball becomes usable partway through the quest line rather than only at
-- the very end. Arc-reward Pokemon (onix, alakazam, lapras) match the
-- "saga" unlock conditions in internal/game/pokedex.go.

DELETE FROM quest_chapters;

INSERT INTO quest_chapters (saga, chapter, title, story, requirement, reward) VALUES
('pewter', 1, 'Pewter Gym - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 8, "skills": ["multiplication","addsub"], "min_difficulty": 1}',
 '{"xp": 300}'),
('pewter', 2, 'Pewter Gym - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 10, "skills": ["division","addsub"], "min_difficulty": 1}',
 '{"xp": 400, "gym_badge": 1}'),
('pewter', 3, 'Pewter Gym - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 10, "skills": ["multiplication","division"], "min_difficulty": 2}',
 '{"xp": 400}'),
('pewter', 4, 'Pewter Gym - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["multiplication","division","addsub"], "min_difficulty": 2}',
 '{"xp": 500, "gym_badge": 2}'),

('cerulean', 1, 'Cerulean Gym - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 10, "skills": ["fractions","place_value"], "min_difficulty": 2}',
 '{"xp": 500}'),
('cerulean', 2, 'Cerulean Gym - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["fractions","patterns"], "min_difficulty": 3}',
 '{"xp": 600, "gym_badge": 3}'),
('cerulean', 3, 'Cerulean Gym - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["place_value","patterns"], "min_difficulty": 3}',
 '{"xp": 600}'),
('cerulean', 4, 'Cerulean Gym - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["fractions","place_value","patterns"], "min_difficulty": 3}',
 '{"xp": 1000, "gym_badge": 4, "pokemon": "onix"}'),

('celadon', 1, 'Celadon Gym - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["word_problems","multiplication"], "min_difficulty": 3}',
 '{"xp": 700}'),
('celadon', 2, 'Celadon Gym - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["logic","division"], "min_difficulty": 3}',
 '{"xp": 700}'),
('celadon', 3, 'Celadon Gym - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["word_problems","logic"], "min_difficulty": 4}',
 '{"xp": 800, "gym_badge": 5}'),
('celadon', 4, 'Celadon Gym - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["multiplication","division","word_problems","logic"], "min_difficulty": 4}',
 '{"xp": 1200, "pokemon": "alakazam"}'),

('fuchsia', 1, 'Fuchsia Gym - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["fractions","place_value"], "min_difficulty": 4}',
 '{"xp": 900}'),
('fuchsia', 2, 'Fuchsia Gym - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["multiplication","division"], "min_difficulty": 5}',
 '{"xp": 1000}'),
('fuchsia', 3, 'Fuchsia Gym - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["word_problems","logic"], "min_difficulty": 5}',
 '{"xp": 1000}'),
('fuchsia', 4, 'Fuchsia Gym - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 18, "skills": ["multiplication","division","fractions","place_value","patterns","word_problems","logic","addsub"], "min_difficulty": 5}',
 '{"xp": 1500, "gym_badge": 6}'),

('cinnabar', 1, 'Cinnabar Gym - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["patterns","place_value"], "min_difficulty": 5}',
 '{"xp": 1200}'),
('cinnabar', 2, 'Cinnabar Gym - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["fractions","logic"], "min_difficulty": 6}',
 '{"xp": 1200, "gym_badge": 7}'),
('cinnabar', 3, 'Cinnabar Gym - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 18, "skills": ["word_problems","multiplication","division"], "min_difficulty": 6}',
 '{"xp": 1500}'),
('cinnabar', 4, 'Cinnabar Gym - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 20, "skills": ["multiplication","division","addsub","fractions","place_value","patterns","word_problems","logic"], "min_difficulty": 6}',
 '{"xp": 2000, "gym_badge": 8, "pokemon": "lapras"}')
ON CONFLICT (saga, chapter) DO NOTHING;
