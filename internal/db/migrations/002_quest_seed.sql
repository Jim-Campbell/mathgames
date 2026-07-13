-- 5 sagas x 4 chapters, in story order (saiyan, namek, android, cell, buu).
-- Titles/story are placeholders; AI batches (kind='story', phase 4) rewrite
-- title/story per saga. Requirement/reward drive gameplay now.
--
-- The 7 dragon balls are scattered one per chapter across the sagas so
-- Shenron becomes summonable partway through the quest line rather than
-- only at the very end. Saga-reward fighters (frieza, cell, majin-buu) match
-- the "saga" unlock conditions in internal/game/fighters.go.

INSERT INTO quest_chapters (saga, chapter, title, story, requirement, reward) VALUES
('saiyan', 1, 'Saiyan Saga - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 8, "skills": ["multiplication","addsub"], "min_difficulty": 1}',
 '{"xp": 300}'),
('saiyan', 2, 'Saiyan Saga - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 10, "skills": ["division","addsub"], "min_difficulty": 1}',
 '{"xp": 400, "dragon_ball": 1}'),
('saiyan', 3, 'Saiyan Saga - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 10, "skills": ["multiplication","division"], "min_difficulty": 2}',
 '{"xp": 400}'),
('saiyan', 4, 'Saiyan Saga - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["multiplication","division","addsub"], "min_difficulty": 2}',
 '{"xp": 500, "dragon_ball": 2}'),

('namek', 1, 'Namek Saga - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 10, "skills": ["fractions","place_value"], "min_difficulty": 2}',
 '{"xp": 500}'),
('namek', 2, 'Namek Saga - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["fractions","patterns"], "min_difficulty": 3}',
 '{"xp": 600, "dragon_ball": 3}'),
('namek', 3, 'Namek Saga - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["place_value","patterns"], "min_difficulty": 3}',
 '{"xp": 600}'),
('namek', 4, 'Namek Saga - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["fractions","place_value","patterns"], "min_difficulty": 3}',
 '{"xp": 1000, "dragon_ball": 4, "fighter": "frieza"}'),

('android', 1, 'Android Saga - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["word_problems","multiplication"], "min_difficulty": 3}',
 '{"xp": 700}'),
('android', 2, 'Android Saga - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["logic","division"], "min_difficulty": 3}',
 '{"xp": 700}'),
('android', 3, 'Android Saga - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["word_problems","logic"], "min_difficulty": 4}',
 '{"xp": 800, "dragon_ball": 5}'),
('android', 4, 'Android Saga - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["multiplication","division","word_problems","logic"], "min_difficulty": 4}',
 '{"xp": 1200}'),

('cell', 1, 'Cell Saga - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 12, "skills": ["fractions","place_value"], "min_difficulty": 4}',
 '{"xp": 900}'),
('cell', 2, 'Cell Saga - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["multiplication","division"], "min_difficulty": 5}',
 '{"xp": 1000}'),
('cell', 3, 'Cell Saga - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["word_problems","logic"], "min_difficulty": 5}',
 '{"xp": 1000}'),
('cell', 4, 'Cell Saga - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 18, "skills": ["multiplication","division","fractions","place_value","patterns","word_problems","logic","addsub"], "min_difficulty": 5}',
 '{"xp": 1500, "dragon_ball": 6, "fighter": "cell"}'),

('buu', 1, 'Buu Saga - Chapter 1', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["patterns","place_value"], "min_difficulty": 5}',
 '{"xp": 1200}'),
('buu', 2, 'Buu Saga - Chapter 2', 'Placeholder story - to be generated.',
 '{"correct": 15, "skills": ["fractions","logic"], "min_difficulty": 6}',
 '{"xp": 1200}'),
('buu', 3, 'Buu Saga - Chapter 3', 'Placeholder story - to be generated.',
 '{"correct": 18, "skills": ["word_problems","multiplication","division"], "min_difficulty": 6}',
 '{"xp": 1500}'),
('buu', 4, 'Buu Saga - Chapter 4', 'Placeholder story - to be generated.',
 '{"correct": 20, "skills": ["multiplication","division","addsub","fractions","place_value","patterns","word_problems","logic"], "min_difficulty": 6}',
 '{"xp": 2000, "dragon_ball": 7, "fighter": "majin-buu"}')
ON CONFLICT (saga, chapter) DO NOTHING;
