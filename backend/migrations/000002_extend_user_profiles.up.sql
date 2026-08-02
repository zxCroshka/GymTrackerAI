SET TIME ZONE 'UTC';

ALTER TABLE user_profiles
    ADD COLUMN sex text,
    ADD COLUMN birth_date date,
    ADD COLUMN goal text,
    ADD COLUMN training_frequency smallint,
    ADD COLUMN sleep_hours_average numeric(3,1),
    ADD CONSTRAINT user_profiles_sex_check CHECK (
        sex IS NULL OR sex IN ('male', 'female', 'other', 'prefer_not_to_say')
    ),
    ADD CONSTRAINT user_profiles_birth_date_check CHECK (
        birth_date IS NULL OR birth_date >= DATE '1900-01-01'
    ),
    ADD CONSTRAINT user_profiles_goal_check CHECK (
        goal IS NULL OR goal IN ('muscle_gain', 'weight_loss', 'recomposition', 'strength', 'maintenance')
    ),
    ADD CONSTRAINT user_profiles_training_frequency_check CHECK (
        training_frequency IS NULL OR training_frequency BETWEEN 1 AND 7
    ),
    ADD CONSTRAINT user_profiles_sleep_hours_check CHECK (
        sleep_hours_average IS NULL OR sleep_hours_average BETWEEN 0 AND 24
    );

CREATE TABLE user_profile_notes (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    position smallint NOT NULL,
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_profile_notes_profile_fk FOREIGN KEY (user_id)
        REFERENCES user_profiles (user_id) ON DELETE CASCADE,
    CONSTRAINT user_profile_notes_user_position_unique UNIQUE (user_id, position),
    CONSTRAINT user_profile_notes_position_check CHECK (position BETWEEN 1 AND 20),
    CONSTRAINT user_profile_notes_content_check CHECK (
        content = btrim(content) AND length(content) BETWEEN 1 AND 1000
    ),
    CONSTRAINT user_profile_notes_updated_at_check CHECK (updated_at >= created_at)
);
