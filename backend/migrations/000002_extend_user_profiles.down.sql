SET TIME ZONE 'UTC';

DROP TABLE IF EXISTS user_profile_notes;

ALTER TABLE user_profiles
    DROP CONSTRAINT IF EXISTS user_profiles_sleep_hours_check,
    DROP CONSTRAINT IF EXISTS user_profiles_training_frequency_check,
    DROP CONSTRAINT IF EXISTS user_profiles_goal_check,
    DROP CONSTRAINT IF EXISTS user_profiles_birth_date_check,
    DROP CONSTRAINT IF EXISTS user_profiles_sex_check,
    DROP COLUMN IF EXISTS sleep_hours_average,
    DROP COLUMN IF EXISTS training_frequency,
    DROP COLUMN IF EXISTS goal,
    DROP COLUMN IF EXISTS birth_date,
    DROP COLUMN IF EXISTS sex;
