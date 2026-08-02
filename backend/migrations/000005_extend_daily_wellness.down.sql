SET TIME ZONE 'UTC';

-- Refuse a rollback that would silently discard nutrition/steps facts or the
-- original observation instant introduced by 000005.
ALTER TABLE daily_wellness
    ADD CONSTRAINT daily_wellness_000005_rollback_check CHECK (
        steps IS NULL
        AND calories_kcal IS NULL
        AND protein_g IS NULL
        AND fat_g IS NULL
        AND carbs_g IS NULL
        AND observed_at = day_start_at
    ) NOT VALID;

ALTER TABLE daily_wellness
    VALIDATE CONSTRAINT daily_wellness_000005_rollback_check;

DROP INDEX daily_wellness_user_observed_idx;

ALTER TABLE daily_wellness
    DROP CONSTRAINT daily_wellness_000005_rollback_check,
    DROP CONSTRAINT daily_wellness_presence_check,
    DROP CONSTRAINT daily_wellness_steps_check,
    DROP CONSTRAINT daily_wellness_calories_check,
    DROP CONSTRAINT daily_wellness_protein_check,
    DROP CONSTRAINT daily_wellness_fat_check,
    DROP CONSTRAINT daily_wellness_carbs_check,
    DROP COLUMN observed_at,
    DROP COLUMN steps,
    DROP COLUMN calories_kcal,
    DROP COLUMN protein_g,
    DROP COLUMN fat_g,
    DROP COLUMN carbs_g,
    ADD CONSTRAINT daily_wellness_presence_check CHECK (
        num_nonnulls(
            sleep_minutes, sleep_quality, energy_level, stress_level,
            soreness_level, mood, resting_heart_rate
        ) > 0
        OR (notes IS NOT NULL AND length(btrim(notes)) > 0)
    );
