SET TIME ZONE 'UTC';

ALTER TABLE daily_wellness
    ADD COLUMN observed_at timestamptz,
    ADD COLUMN steps integer,
    ADD COLUMN calories_kcal numeric(8,2),
    ADD COLUMN protein_g numeric(8,2),
    ADD COLUMN fat_g numeric(8,2),
    ADD COLUMN carbs_g numeric(8,2);

UPDATE daily_wellness SET observed_at = day_start_at WHERE observed_at IS NULL;

ALTER TABLE daily_wellness
    ALTER COLUMN observed_at SET NOT NULL,
    DROP CONSTRAINT daily_wellness_presence_check,
    ADD CONSTRAINT daily_wellness_presence_check CHECK (
        num_nonnulls(
            sleep_minutes, sleep_quality, energy_level, stress_level,
            soreness_level, mood, resting_heart_rate, steps, calories_kcal,
            protein_g, fat_g, carbs_g
        ) > 0
        OR (notes IS NOT NULL AND length(btrim(notes)) > 0)
    ) NOT VALID,
    ADD CONSTRAINT daily_wellness_steps_check CHECK (steps IS NULL OR steps BETWEEN 0 AND 1000000) NOT VALID,
    ADD CONSTRAINT daily_wellness_calories_check CHECK (calories_kcal IS NULL OR calories_kcal BETWEEN 0 AND 50000) NOT VALID,
    ADD CONSTRAINT daily_wellness_protein_check CHECK (protein_g IS NULL OR protein_g BETWEEN 0 AND 5000) NOT VALID,
    ADD CONSTRAINT daily_wellness_fat_check CHECK (fat_g IS NULL OR fat_g BETWEEN 0 AND 5000) NOT VALID,
    ADD CONSTRAINT daily_wellness_carbs_check CHECK (carbs_g IS NULL OR carbs_g BETWEEN 0 AND 5000) NOT VALID;

ALTER TABLE daily_wellness
    VALIDATE CONSTRAINT daily_wellness_presence_check,
    VALIDATE CONSTRAINT daily_wellness_steps_check,
    VALIDATE CONSTRAINT daily_wellness_calories_check,
    VALIDATE CONSTRAINT daily_wellness_protein_check,
    VALIDATE CONSTRAINT daily_wellness_fat_check,
    VALIDATE CONSTRAINT daily_wellness_carbs_check;

CREATE INDEX daily_wellness_user_observed_idx
    ON daily_wellness (user_id, observed_at DESC, id DESC);
