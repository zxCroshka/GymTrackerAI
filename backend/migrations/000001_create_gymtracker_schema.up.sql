SET TIME ZONE 'UTC';

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    password_hash text NOT NULL,
    status text NOT NULL DEFAULT 'active',
    auth_version integer NOT NULL DEFAULT 1,
    email_verified_at timestamptz,
    disabled_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT users_email_format_check CHECK (
        email = lower(btrim(email)) AND length(email) BETWEEN 3 AND 320
    ),
    CONSTRAINT users_password_hash_check CHECK (length(password_hash) BETWEEN 1 AND 1024),
    CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled', 'deleted')),
    CONSTRAINT users_auth_version_check CHECK (auth_version > 0),
    CONSTRAINT users_status_timestamps_check CHECK (
        (status = 'active' AND disabled_at IS NULL AND deleted_at IS NULL)
        OR (status = 'disabled' AND disabled_at IS NOT NULL AND deleted_at IS NULL)
        OR (status = 'deleted' AND deleted_at IS NOT NULL)
    ),
    CONSTRAINT users_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX users_email_uidx ON users (email);
CREATE INDEX users_status_idx ON users (status);

CREATE TABLE user_profiles (
    user_id uuid PRIMARY KEY,
    display_name text,
    timezone text NOT NULL DEFAULT 'UTC',
    locale text NOT NULL DEFAULT 'ru-RU',
    preferred_unit_system text NOT NULL DEFAULT 'metric',
    height_cm numeric(6,2),
    experience_level text,
    training_goal text,
    ai_coach_enabled boolean NOT NULL DEFAULT false,
    ai_notice_version text,
    ai_enabled_at timestamptz,
    ai_disabled_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_profiles_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT user_profiles_display_name_check CHECK (
        display_name IS NULL OR (display_name = btrim(display_name) AND length(display_name) BETWEEN 1 AND 100)
    ),
    CONSTRAINT user_profiles_timezone_check CHECK (timezone = btrim(timezone) AND length(timezone) BETWEEN 1 AND 255),
    CONSTRAINT user_profiles_locale_check CHECK (locale = btrim(locale) AND length(locale) BETWEEN 2 AND 35),
    CONSTRAINT user_profiles_unit_system_check CHECK (preferred_unit_system IN ('metric', 'imperial')),
    CONSTRAINT user_profiles_height_check CHECK (height_cm IS NULL OR height_cm BETWEEN 50 AND 300),
    CONSTRAINT user_profiles_experience_check CHECK (
        experience_level IS NULL OR experience_level IN ('beginner', 'intermediate', 'advanced')
    ),
    CONSTRAINT user_profiles_training_goal_check CHECK (training_goal IS NULL OR length(training_goal) <= 2000),
    CONSTRAINT user_profiles_ai_notice_check CHECK (
        NOT ai_coach_enabled
        OR (
            ai_notice_version IS NOT NULL
            AND length(btrim(ai_notice_version)) BETWEEN 1 AND 100
            AND ai_enabled_at IS NOT NULL
        )
    ),
    CONSTRAINT user_profiles_version_check CHECK (version > 0),
    CONSTRAINT user_profiles_updated_at_check CHECK (updated_at >= created_at)
);

CREATE TABLE refresh_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    jti uuid NOT NULL,
    family_id uuid NOT NULL,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz,
    replaced_by_token_id uuid,
    revocation_reason text,
    created_ip inet,
    user_agent text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT refresh_tokens_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT refresh_tokens_id_user_family_unique UNIQUE (id, user_id, family_id),
    CONSTRAINT refresh_tokens_jti_unique UNIQUE (jti),
    CONSTRAINT refresh_tokens_hash_unique UNIQUE (token_hash),
    CONSTRAINT refresh_tokens_hash_length_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT refresh_tokens_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT refresh_tokens_replacement_check CHECK (replaced_by_token_id IS NULL OR replaced_by_token_id <> id),
    CONSTRAINT refresh_tokens_revocation_check CHECK (
        (revoked_at IS NULL AND revocation_reason IS NULL)
        OR (
            revoked_at IS NOT NULL
            AND revocation_reason IS NOT NULL
            AND length(btrim(revocation_reason)) BETWEEN 1 AND 100
        )
    ),
    CONSTRAINT refresh_tokens_user_agent_check CHECK (user_agent IS NULL OR length(user_agent) <= 1000),
    CONSTRAINT refresh_tokens_updated_at_check CHECK (updated_at >= created_at)
);

ALTER TABLE refresh_tokens
    ADD CONSTRAINT refresh_tokens_replaced_by_fk
    FOREIGN KEY (replaced_by_token_id, user_id, family_id)
    REFERENCES refresh_tokens (id, user_id, family_id)
    ON DELETE NO ACTION;

CREATE UNIQUE INDEX refresh_tokens_replaced_by_uidx
    ON refresh_tokens (replaced_by_token_id)
    WHERE replaced_by_token_id IS NOT NULL;
CREATE INDEX refresh_tokens_active_user_expiry_idx
    ON refresh_tokens (user_id, expires_at)
    WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_user_family_created_idx
    ON refresh_tokens (user_id, family_id, created_at DESC);

CREATE TABLE exercises (
    id uuid PRIMARY KEY,
    owner_user_id uuid,
    name text NOT NULL,
    description text,
    instructions text,
    primary_muscle_group text,
    equipment text,
    movement_pattern text,
    is_unilateral boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT 1,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT exercises_owner_fk FOREIGN KEY (owner_user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT exercises_name_check CHECK (name = btrim(name) AND length(name) BETWEEN 1 AND 200),
    CONSTRAINT exercises_description_check CHECK (description IS NULL OR length(description) <= 4000),
    CONSTRAINT exercises_instructions_check CHECK (instructions IS NULL OR length(instructions) <= 8000),
    CONSTRAINT exercises_muscle_group_check CHECK (
        primary_muscle_group IS NULL OR length(btrim(primary_muscle_group)) BETWEEN 1 AND 100
    ),
    CONSTRAINT exercises_equipment_check CHECK (equipment IS NULL OR length(btrim(equipment)) BETWEEN 1 AND 100),
    CONSTRAINT exercises_movement_pattern_check CHECK (
        movement_pattern IS NULL OR length(btrim(movement_pattern)) BETWEEN 1 AND 100
    ),
    CONSTRAINT exercises_version_check CHECK (version > 0),
    CONSTRAINT exercises_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX exercises_global_name_uidx
    ON exercises (lower(name))
    WHERE owner_user_id IS NULL;
CREATE UNIQUE INDEX exercises_owner_active_name_uidx
    ON exercises (owner_user_id, lower(name))
    WHERE owner_user_id IS NOT NULL AND archived_at IS NULL;
CREATE INDEX exercises_owner_archived_idx ON exercises (owner_user_id, archived_at);
CREATE INDEX exercises_normalized_name_idx ON exercises (lower(name));

CREATE TABLE programs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    goal text,
    status text NOT NULL DEFAULT 'draft',
    version bigint NOT NULL DEFAULT 1,
    activated_at timestamptz,
    inactivated_at timestamptz,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT programs_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT programs_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT programs_name_check CHECK (name = btrim(name) AND length(name) BETWEEN 1 AND 200),
    CONSTRAINT programs_description_check CHECK (description IS NULL OR length(description) <= 4000),
    CONSTRAINT programs_goal_check CHECK (goal IS NULL OR length(goal) <= 2000),
    CONSTRAINT programs_status_check CHECK (status IN ('draft', 'active', 'inactive', 'archived')),
    CONSTRAINT programs_version_check CHECK (version > 0),
    CONSTRAINT programs_lifecycle_check CHECK (
        (status <> 'active' OR activated_at IS NOT NULL)
        AND (status <> 'inactive' OR inactivated_at IS NOT NULL)
        AND ((status = 'archived') = (archived_at IS NOT NULL))
    ),
    CONSTRAINT programs_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX programs_one_active_per_user_uidx ON programs (user_id) WHERE status = 'active';
CREATE INDEX programs_user_status_updated_idx ON programs (user_id, status, updated_at DESC);

CREATE TABLE program_days (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    program_id uuid NOT NULL,
    position smallint NOT NULL,
    name text NOT NULL,
    notes text,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT program_days_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT program_days_program_fk FOREIGN KEY (program_id, user_id)
        REFERENCES programs (id, user_id) ON DELETE CASCADE,
    CONSTRAINT program_days_position_check CHECK (position > 0),
    CONSTRAINT program_days_name_check CHECK (name = btrim(name) AND length(name) BETWEEN 1 AND 200),
    CONSTRAINT program_days_notes_check CHECK (notes IS NULL OR length(notes) <= 4000),
    CONSTRAINT program_days_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX program_days_active_position_uidx
    ON program_days (program_id, position)
    WHERE archived_at IS NULL;
CREATE INDEX program_days_program_user_idx ON program_days (program_id, user_id);
CREATE INDEX program_days_user_program_idx ON program_days (user_id, program_id);

CREATE TABLE program_day_exercises (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    program_day_id uuid NOT NULL,
    exercise_id uuid NOT NULL,
    position smallint NOT NULL,
    target_sets smallint NOT NULL,
    target_reps_min smallint,
    target_reps_max smallint,
    target_rir numeric(3,1),
    rest_seconds integer,
    notes text,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT program_day_exercises_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT program_day_exercises_day_fk FOREIGN KEY (program_day_id, user_id)
        REFERENCES program_days (id, user_id) ON DELETE CASCADE,
    CONSTRAINT program_day_exercises_exercise_fk FOREIGN KEY (exercise_id)
        REFERENCES exercises (id) ON DELETE NO ACTION,
    CONSTRAINT program_day_exercises_position_check CHECK (position > 0),
    CONSTRAINT program_day_exercises_target_sets_check CHECK (target_sets BETWEEN 1 AND 100),
    CONSTRAINT program_day_exercises_target_reps_check CHECK (
        (target_reps_min IS NULL OR target_reps_min >= 0)
        AND (target_reps_max IS NULL OR target_reps_max >= 0)
        AND (target_reps_min IS NULL OR target_reps_max IS NULL OR target_reps_max >= target_reps_min)
    ),
    CONSTRAINT program_day_exercises_target_rir_check CHECK (target_rir IS NULL OR target_rir BETWEEN 0 AND 10),
    CONSTRAINT program_day_exercises_rest_check CHECK (rest_seconds IS NULL OR rest_seconds BETWEEN 0 AND 86400),
    CONSTRAINT program_day_exercises_notes_check CHECK (notes IS NULL OR length(notes) <= 4000),
    CONSTRAINT program_day_exercises_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX program_day_exercises_active_position_uidx
    ON program_day_exercises (program_day_id, position)
    WHERE archived_at IS NULL;
CREATE INDEX program_day_exercises_day_user_idx ON program_day_exercises (program_day_id, user_id);
CREATE INDEX program_day_exercises_exercise_idx ON program_day_exercises (exercise_id);
CREATE INDEX program_day_exercises_user_exercise_idx ON program_day_exercises (user_id, exercise_id);
CREATE INDEX program_day_exercises_day_archived_idx ON program_day_exercises (program_day_id, archived_at);

CREATE TABLE workouts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    source_program_day_id uuid,
    source_program_version bigint,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'planned',
    scheduled_at timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    cancelled_at timestamptz,
    notes text,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT workouts_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT workouts_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT workouts_source_day_fk FOREIGN KEY (source_program_day_id, user_id)
        REFERENCES program_days (id, user_id) ON DELETE NO ACTION,
    CONSTRAINT workouts_source_check CHECK ((source_program_day_id IS NULL) = (source_program_version IS NULL)),
    CONSTRAINT workouts_source_version_check CHECK (source_program_version IS NULL OR source_program_version > 0),
    CONSTRAINT workouts_name_check CHECK (name = btrim(name) AND length(name) BETWEEN 1 AND 200),
    CONSTRAINT workouts_status_check CHECK (status IN ('planned', 'in_progress', 'completed', 'cancelled')),
    CONSTRAINT workouts_lifecycle_check CHECK (
        (status = 'planned' AND started_at IS NULL AND completed_at IS NULL AND cancelled_at IS NULL)
        OR (status = 'in_progress' AND started_at IS NOT NULL AND completed_at IS NULL AND cancelled_at IS NULL)
        OR (status = 'completed' AND started_at IS NOT NULL AND completed_at IS NOT NULL AND cancelled_at IS NULL)
        OR (status = 'cancelled' AND completed_at IS NULL AND cancelled_at IS NOT NULL)
    ),
    CONSTRAINT workouts_completion_order_check CHECK (
        completed_at IS NULL OR (started_at IS NOT NULL AND completed_at >= started_at)
    ),
    CONSTRAINT workouts_notes_check CHECK (notes IS NULL OR length(notes) <= 4000),
    CONSTRAINT workouts_version_check CHECK (version > 0),
    CONSTRAINT workouts_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX workouts_one_in_progress_per_user_uidx ON workouts (user_id) WHERE status = 'in_progress';
CREATE INDEX workouts_user_started_idx ON workouts (user_id, started_at DESC, id DESC);
CREATE INDEX workouts_user_status_scheduled_idx ON workouts (user_id, status, scheduled_at);
CREATE INDEX workouts_source_day_user_idx ON workouts (source_program_day_id, user_id);

CREATE TABLE workout_exercises (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    workout_id uuid NOT NULL,
    exercise_id uuid NOT NULL,
    source_program_day_exercise_id uuid,
    position smallint NOT NULL,
    exercise_name_snapshot text NOT NULL,
    notes text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT workout_exercises_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT workout_exercises_workout_fk FOREIGN KEY (workout_id, user_id)
        REFERENCES workouts (id, user_id) ON DELETE CASCADE,
    CONSTRAINT workout_exercises_exercise_fk FOREIGN KEY (exercise_id)
        REFERENCES exercises (id) ON DELETE NO ACTION,
    CONSTRAINT workout_exercises_source_item_fk FOREIGN KEY (source_program_day_exercise_id, user_id)
        REFERENCES program_day_exercises (id, user_id) ON DELETE NO ACTION,
    CONSTRAINT workout_exercises_position_unique UNIQUE (workout_id, position),
    CONSTRAINT workout_exercises_position_check CHECK (position > 0),
    CONSTRAINT workout_exercises_snapshot_check CHECK (
        exercise_name_snapshot = btrim(exercise_name_snapshot)
        AND length(exercise_name_snapshot) BETWEEN 1 AND 200
    ),
    CONSTRAINT workout_exercises_notes_check CHECK (notes IS NULL OR length(notes) <= 4000),
    CONSTRAINT workout_exercises_updated_at_check CHECK (updated_at >= created_at)
);

CREATE INDEX workout_exercises_workout_user_idx ON workout_exercises (workout_id, user_id);
CREATE INDEX workout_exercises_exercise_idx ON workout_exercises (exercise_id);
CREATE INDEX workout_exercises_source_item_user_idx
    ON workout_exercises (source_program_day_exercise_id, user_id);
CREATE INDEX workout_exercises_user_exercise_idx ON workout_exercises (user_id, exercise_id);

CREATE TABLE workout_sets (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    workout_exercise_id uuid NOT NULL,
    position smallint NOT NULL,
    set_type text NOT NULL DEFAULT 'working',
    status text NOT NULL DEFAULT 'planned',
    target_weight_kg numeric(8,3),
    target_reps_min smallint,
    target_reps_max smallint,
    target_rir numeric(3,1),
    weight_kg numeric(8,3),
    reps smallint,
    rir numeric(3,1),
    completed_at timestamptz,
    notes text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT workout_sets_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT workout_sets_exercise_fk FOREIGN KEY (workout_exercise_id, user_id)
        REFERENCES workout_exercises (id, user_id) ON DELETE CASCADE,
    CONSTRAINT workout_sets_position_unique UNIQUE (workout_exercise_id, position),
    CONSTRAINT workout_sets_position_check CHECK (position > 0),
    CONSTRAINT workout_sets_type_check CHECK (set_type IN ('warmup', 'working', 'backoff', 'drop', 'failure')),
    CONSTRAINT workout_sets_status_check CHECK (status IN ('planned', 'completed', 'skipped')),
    CONSTRAINT workout_sets_target_weight_check CHECK (target_weight_kg IS NULL OR target_weight_kg >= 0),
    CONSTRAINT workout_sets_target_reps_check CHECK (
        (target_reps_min IS NULL OR target_reps_min >= 0)
        AND (target_reps_max IS NULL OR target_reps_max >= 0)
        AND (target_reps_min IS NULL OR target_reps_max IS NULL OR target_reps_max >= target_reps_min)
    ),
    CONSTRAINT workout_sets_target_rir_check CHECK (target_rir IS NULL OR target_rir BETWEEN 0 AND 10),
    CONSTRAINT workout_sets_actual_check CHECK (
        (status = 'completed' AND weight_kg IS NOT NULL AND weight_kg >= 0 AND reps IS NOT NULL AND reps >= 0 AND completed_at IS NOT NULL)
        OR (status IN ('planned', 'skipped') AND weight_kg IS NULL AND reps IS NULL AND rir IS NULL AND completed_at IS NULL)
    ),
    CONSTRAINT workout_sets_rir_check CHECK (rir IS NULL OR rir BETWEEN 0 AND 10),
    CONSTRAINT workout_sets_notes_check CHECK (notes IS NULL OR length(notes) <= 4000),
    CONSTRAINT workout_sets_updated_at_check CHECK (updated_at >= created_at)
);

CREATE INDEX workout_sets_user_completed_idx ON workout_sets (user_id, completed_at DESC);
CREATE INDEX workout_sets_exercise_user_idx ON workout_sets (workout_exercise_id, user_id);

CREATE TABLE body_measurements (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    measured_at timestamptz NOT NULL,
    weight_kg numeric(8,3),
    body_fat_percent numeric(5,2),
    neck_cm numeric(6,2),
    chest_cm numeric(6,2),
    waist_cm numeric(6,2),
    hips_cm numeric(6,2),
    left_upper_arm_cm numeric(6,2),
    right_upper_arm_cm numeric(6,2),
    left_thigh_cm numeric(6,2),
    right_thigh_cm numeric(6,2),
    left_calf_cm numeric(6,2),
    right_calf_cm numeric(6,2),
    notes text,
    source text NOT NULL DEFAULT 'manual',
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT body_measurements_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT body_measurements_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT body_measurements_user_measured_unique UNIQUE (user_id, measured_at),
    CONSTRAINT body_measurements_presence_check CHECK (
        num_nonnulls(
            weight_kg, body_fat_percent, neck_cm, chest_cm, waist_cm, hips_cm,
            left_upper_arm_cm, right_upper_arm_cm, left_thigh_cm, right_thigh_cm,
            left_calf_cm, right_calf_cm
        ) > 0
    ),
    CONSTRAINT body_measurements_weight_check CHECK (weight_kg IS NULL OR weight_kg BETWEEN 20 AND 700),
    CONSTRAINT body_measurements_body_fat_check CHECK (body_fat_percent IS NULL OR body_fat_percent BETWEEN 0 AND 100),
    CONSTRAINT body_measurements_neck_check CHECK (neck_cm IS NULL OR neck_cm BETWEEN 5 AND 200),
    CONSTRAINT body_measurements_chest_check CHECK (chest_cm IS NULL OR chest_cm BETWEEN 5 AND 400),
    CONSTRAINT body_measurements_waist_check CHECK (waist_cm IS NULL OR waist_cm BETWEEN 5 AND 400),
    CONSTRAINT body_measurements_hips_check CHECK (hips_cm IS NULL OR hips_cm BETWEEN 5 AND 400),
    CONSTRAINT body_measurements_left_arm_check CHECK (left_upper_arm_cm IS NULL OR left_upper_arm_cm BETWEEN 5 AND 200),
    CONSTRAINT body_measurements_right_arm_check CHECK (right_upper_arm_cm IS NULL OR right_upper_arm_cm BETWEEN 5 AND 200),
    CONSTRAINT body_measurements_left_thigh_check CHECK (left_thigh_cm IS NULL OR left_thigh_cm BETWEEN 5 AND 250),
    CONSTRAINT body_measurements_right_thigh_check CHECK (right_thigh_cm IS NULL OR right_thigh_cm BETWEEN 5 AND 250),
    CONSTRAINT body_measurements_left_calf_check CHECK (left_calf_cm IS NULL OR left_calf_cm BETWEEN 5 AND 200),
    CONSTRAINT body_measurements_right_calf_check CHECK (right_calf_cm IS NULL OR right_calf_cm BETWEEN 5 AND 200),
    CONSTRAINT body_measurements_notes_check CHECK (notes IS NULL OR length(notes) <= 4000),
    CONSTRAINT body_measurements_source_check CHECK (source IN ('manual', 'import')),
    CONSTRAINT body_measurements_version_check CHECK (version > 0),
    CONSTRAINT body_measurements_updated_at_check CHECK (updated_at >= created_at)
);

CREATE INDEX body_measurements_user_measured_idx ON body_measurements (user_id, measured_at DESC, id DESC);

CREATE TABLE daily_wellness (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    day_start_at timestamptz NOT NULL,
    timezone_at_entry text NOT NULL,
    sleep_minutes smallint,
    sleep_quality smallint,
    energy_level smallint,
    stress_level smallint,
    soreness_level smallint,
    mood smallint,
    resting_heart_rate smallint,
    notes text,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT daily_wellness_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT daily_wellness_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT daily_wellness_user_day_unique UNIQUE (user_id, day_start_at),
    CONSTRAINT daily_wellness_timezone_check CHECK (
        timezone_at_entry = btrim(timezone_at_entry) AND length(timezone_at_entry) BETWEEN 1 AND 255
    ),
    CONSTRAINT daily_wellness_presence_check CHECK (
        num_nonnulls(sleep_minutes, sleep_quality, energy_level, stress_level, soreness_level, mood, resting_heart_rate) > 0
        OR (notes IS NOT NULL AND length(btrim(notes)) > 0)
    ),
    CONSTRAINT daily_wellness_sleep_minutes_check CHECK (sleep_minutes IS NULL OR sleep_minutes BETWEEN 0 AND 1440),
    CONSTRAINT daily_wellness_sleep_quality_check CHECK (sleep_quality IS NULL OR sleep_quality BETWEEN 1 AND 5),
    CONSTRAINT daily_wellness_energy_check CHECK (energy_level IS NULL OR energy_level BETWEEN 1 AND 5),
    CONSTRAINT daily_wellness_stress_check CHECK (stress_level IS NULL OR stress_level BETWEEN 1 AND 5),
    CONSTRAINT daily_wellness_soreness_check CHECK (soreness_level IS NULL OR soreness_level BETWEEN 1 AND 5),
    CONSTRAINT daily_wellness_mood_check CHECK (mood IS NULL OR mood BETWEEN 1 AND 5),
    CONSTRAINT daily_wellness_heart_rate_check CHECK (resting_heart_rate IS NULL OR resting_heart_rate BETWEEN 20 AND 250),
    CONSTRAINT daily_wellness_notes_check CHECK (notes IS NULL OR length(notes) <= 4000),
    CONSTRAINT daily_wellness_version_check CHECK (version > 0),
    CONSTRAINT daily_wellness_updated_at_check CHECK (updated_at >= created_at)
);

CREATE INDEX daily_wellness_user_day_idx ON daily_wellness (user_id, day_start_at DESC);

CREATE TABLE personal_records (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    exercise_id uuid NOT NULL,
    workout_set_id uuid NOT NULL,
    record_type text NOT NULL,
    value numeric(14,3) NOT NULL,
    calculation_version text NOT NULL,
    formula text,
    achieved_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT personal_records_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT personal_records_exercise_fk FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE NO ACTION,
    CONSTRAINT personal_records_set_fk FOREIGN KEY (workout_set_id, user_id)
        REFERENCES workout_sets (id, user_id) ON DELETE CASCADE,
    CONSTRAINT personal_records_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT personal_records_source_type_unique UNIQUE (user_id, workout_set_id, record_type),
    CONSTRAINT personal_records_type_check CHECK (
        record_type IN ('max_weight', 'max_reps', 'estimated_1rm', 'max_set_volume')
    ),
    CONSTRAINT personal_records_value_check CHECK (value >= 0),
    CONSTRAINT personal_records_calculation_version_check CHECK (
        length(btrim(calculation_version)) BETWEEN 1 AND 100
    ),
    CONSTRAINT personal_records_formula_check CHECK (
        (record_type = 'estimated_1rm' AND formula IS NOT NULL AND length(btrim(formula)) BETWEEN 1 AND 100)
        OR (record_type <> 'estimated_1rm' AND formula IS NULL)
    )
);

CREATE INDEX personal_records_set_user_idx ON personal_records (workout_set_id, user_id);
CREATE INDEX personal_records_exercise_idx ON personal_records (exercise_id);
CREATE INDEX personal_records_user_exercise_type_value_idx
    ON personal_records (user_id, exercise_id, record_type, value DESC);
CREATE INDEX personal_records_user_achieved_idx ON personal_records (user_id, achieved_at DESC);

CREATE TABLE coach_conversations (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    title text,
    status text NOT NULL DEFAULT 'active',
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT coach_conversations_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT coach_conversations_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT coach_conversations_title_check CHECK (title IS NULL OR length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT coach_conversations_status_check CHECK (status IN ('active', 'archived')),
    CONSTRAINT coach_conversations_version_check CHECK (version > 0),
    CONSTRAINT coach_conversations_updated_at_check CHECK (updated_at >= created_at)
);

CREATE INDEX coach_conversations_user_updated_idx
    ON coach_conversations (user_id, updated_at DESC, id DESC);

CREATE TABLE coach_messages (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    sequence_number bigint NOT NULL,
    role text NOT NULL,
    status text NOT NULL,
    content text,
    client_message_id uuid,
    model text,
    provider_response_id text,
    prompt_version text,
    input_tokens integer,
    output_tokens integer,
    attempt_count smallint NOT NULL DEFAULT 0,
    processing_attempt_id uuid,
    processing_started_at timestamptz,
    lease_expires_at timestamptz,
    next_attempt_at timestamptz,
    retryable boolean NOT NULL DEFAULT false,
    completed_at timestamptz,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT coach_messages_conversation_fk FOREIGN KEY (conversation_id, user_id)
        REFERENCES coach_conversations (id, user_id) ON DELETE CASCADE,
    CONSTRAINT coach_messages_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT coach_messages_id_conversation_user_unique UNIQUE (id, conversation_id, user_id),
    CONSTRAINT coach_messages_sequence_unique UNIQUE (conversation_id, sequence_number),
    CONSTRAINT coach_messages_sequence_check CHECK (sequence_number > 0),
    CONSTRAINT coach_messages_role_check CHECK (role IN ('user', 'assistant')),
    CONSTRAINT coach_messages_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    CONSTRAINT coach_messages_client_id_check CHECK (
        (role = 'user' AND client_message_id IS NOT NULL)
        OR (role = 'assistant' AND client_message_id IS NULL)
    ),
    CONSTRAINT coach_messages_content_length_check CHECK (
        content IS NULL
        OR (length(btrim(content)) > 0 AND length(content) <= CASE WHEN role = 'user' THEN 4000 ELSE 16000 END)
    ),
    CONSTRAINT coach_messages_token_counts_check CHECK (
        (input_tokens IS NULL OR input_tokens >= 0) AND (output_tokens IS NULL OR output_tokens >= 0)
    ),
    CONSTRAINT coach_messages_attempt_count_check CHECK (attempt_count BETWEEN 0 AND 100),
    CONSTRAINT coach_messages_lifecycle_check CHECK (
        (
            role = 'user' AND status = 'completed' AND content IS NOT NULL AND completed_at IS NOT NULL
            AND processing_attempt_id IS NULL AND processing_started_at IS NULL AND lease_expires_at IS NULL
            AND error_code IS NULL
        )
        OR (
            role = 'assistant' AND status = 'pending' AND content IS NULL AND completed_at IS NULL
            AND processing_attempt_id IS NULL AND processing_started_at IS NULL AND lease_expires_at IS NULL
            AND error_code IS NULL
        )
        OR (
            role = 'assistant' AND status = 'processing' AND content IS NULL AND completed_at IS NULL
            AND processing_attempt_id IS NOT NULL AND processing_started_at IS NOT NULL AND lease_expires_at IS NOT NULL
            AND lease_expires_at > processing_started_at AND error_code IS NULL
        )
        OR (
            role = 'assistant' AND status = 'completed' AND content IS NOT NULL AND completed_at IS NOT NULL
            AND processing_attempt_id IS NULL AND processing_started_at IS NULL AND lease_expires_at IS NULL
            AND error_code IS NULL
        )
        OR (
            role = 'assistant' AND status IN ('failed', 'cancelled') AND content IS NULL AND completed_at IS NOT NULL
            AND processing_attempt_id IS NULL AND processing_started_at IS NULL AND lease_expires_at IS NULL
            AND error_code IS NOT NULL AND length(btrim(error_code)) BETWEEN 1 AND 100
        )
    ),
    CONSTRAINT coach_messages_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX coach_messages_client_message_uidx
    ON coach_messages (conversation_id, client_message_id)
    WHERE client_message_id IS NOT NULL;
CREATE INDEX coach_messages_user_created_idx ON coach_messages (user_id, created_at DESC);
CREATE INDEX coach_messages_pending_work_idx ON coach_messages (status, next_attempt_at, lease_expires_at);

CREATE TABLE coach_tool_calls (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    assistant_message_id uuid NOT NULL,
    processing_attempt_id uuid NOT NULL,
    provider_tool_call_id text,
    tool_name text NOT NULL,
    arguments_summary jsonb NOT NULL,
    arguments_digest bytea NOT NULL,
    result_summary jsonb,
    result_digest bytea,
    status text NOT NULL,
    error_code text,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT coach_tool_calls_conversation_fk FOREIGN KEY (conversation_id, user_id)
        REFERENCES coach_conversations (id, user_id) ON DELETE CASCADE,
    CONSTRAINT coach_tool_calls_message_fk FOREIGN KEY (assistant_message_id, conversation_id, user_id)
        REFERENCES coach_messages (id, conversation_id, user_id) ON DELETE CASCADE,
    CONSTRAINT coach_tool_calls_arguments_object_check CHECK (jsonb_typeof(arguments_summary) = 'object'),
    CONSTRAINT coach_tool_calls_arguments_digest_check CHECK (octet_length(arguments_digest) = 32),
    CONSTRAINT coach_tool_calls_result_object_check CHECK (
        result_summary IS NULL OR jsonb_typeof(result_summary) = 'object'
    ),
    CONSTRAINT coach_tool_calls_result_digest_check CHECK (
        result_digest IS NULL OR octet_length(result_digest) = 32
    ),
    CONSTRAINT coach_tool_calls_result_pair_check CHECK ((result_summary IS NULL) = (result_digest IS NULL)),
    CONSTRAINT coach_tool_calls_tool_name_check CHECK (length(btrim(tool_name)) BETWEEN 1 AND 100),
    CONSTRAINT coach_tool_calls_status_check CHECK (status IN ('requested', 'succeeded', 'failed', 'denied')),
    CONSTRAINT coach_tool_calls_lifecycle_check CHECK (
        (status = 'requested' AND finished_at IS NULL AND result_summary IS NULL AND error_code IS NULL)
        OR (status = 'succeeded' AND finished_at IS NOT NULL AND result_summary IS NOT NULL AND error_code IS NULL)
        OR (
            status IN ('failed', 'denied') AND finished_at IS NOT NULL AND result_summary IS NULL
            AND error_code IS NOT NULL AND length(btrim(error_code)) BETWEEN 1 AND 100
        )
    ),
    CONSTRAINT coach_tool_calls_finished_order_check CHECK (finished_at IS NULL OR finished_at >= started_at),
    CONSTRAINT coach_tool_calls_updated_at_check CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX coach_tool_calls_provider_id_uidx
    ON coach_tool_calls (conversation_id, provider_tool_call_id)
    WHERE provider_tool_call_id IS NOT NULL;
CREATE INDEX coach_tool_calls_conversation_user_idx ON coach_tool_calls (conversation_id, user_id);
CREATE INDEX coach_tool_calls_user_created_idx ON coach_tool_calls (user_id, created_at DESC);
CREATE INDEX coach_tool_calls_message_created_idx ON coach_tool_calls (assistant_message_id, created_at);

CREATE TABLE coach_recommendations (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    source_message_id uuid NOT NULL,
    target_program_id uuid NOT NULL,
    recommendation_type text NOT NULL DEFAULT 'program_change',
    summary text NOT NULL,
    rationale text NOT NULL,
    payload_schema_version smallint NOT NULL DEFAULT 1,
    payload jsonb NOT NULL,
    proposal_hash bytea NOT NULL,
    expected_program_version bigint NOT NULL,
    status text NOT NULL DEFAULT 'proposed',
    expires_at timestamptz NOT NULL,
    decided_at timestamptz,
    reviewed_by_user_id uuid,
    applied_at timestamptz,
    applied_program_version bigint,
    rejection_reason text,
    model text,
    prompt_version text NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT coach_recommendations_conversation_fk FOREIGN KEY (conversation_id, user_id)
        REFERENCES coach_conversations (id, user_id) ON DELETE CASCADE,
    CONSTRAINT coach_recommendations_source_message_fk FOREIGN KEY (source_message_id, conversation_id, user_id)
        REFERENCES coach_messages (id, conversation_id, user_id) ON DELETE CASCADE,
    CONSTRAINT coach_recommendations_program_fk FOREIGN KEY (target_program_id, user_id)
        REFERENCES programs (id, user_id) ON DELETE NO ACTION,
    CONSTRAINT coach_recommendations_reviewer_fk FOREIGN KEY (reviewed_by_user_id)
        REFERENCES users (id) ON DELETE NO ACTION,
    CONSTRAINT coach_recommendations_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT coach_recommendations_source_target_version_unique
        UNIQUE (user_id, source_message_id, target_program_id, expected_program_version),
    CONSTRAINT coach_recommendations_type_check CHECK (recommendation_type = 'program_change'),
    CONSTRAINT coach_recommendations_summary_check CHECK (length(btrim(summary)) BETWEEN 1 AND 500),
    CONSTRAINT coach_recommendations_rationale_check CHECK (length(btrim(rationale)) BETWEEN 1 AND 4000),
    CONSTRAINT coach_recommendations_payload_version_check CHECK (payload_schema_version > 0),
    CONSTRAINT coach_recommendations_payload_object_check CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT coach_recommendations_hash_check CHECK (octet_length(proposal_hash) = 32),
    CONSTRAINT coach_recommendations_expected_version_check CHECK (expected_program_version > 0),
    CONSTRAINT coach_recommendations_status_check CHECK (
        status IN ('proposed', 'applied', 'rejected', 'expired', 'superseded')
    ),
    CONSTRAINT coach_recommendations_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT coach_recommendations_reviewer_owner_check CHECK (
        reviewed_by_user_id IS NULL OR reviewed_by_user_id = user_id
    ),
    CONSTRAINT coach_recommendations_lifecycle_check CHECK (
        (
            status = 'proposed' AND decided_at IS NULL AND reviewed_by_user_id IS NULL
            AND applied_at IS NULL AND applied_program_version IS NULL AND rejection_reason IS NULL
        )
        OR (
            status = 'applied' AND decided_at IS NOT NULL AND reviewed_by_user_id IS NOT NULL
            AND applied_at IS NOT NULL AND applied_program_version IS NOT NULL AND applied_program_version > 0
            AND rejection_reason IS NULL
        )
        OR (
            status = 'rejected' AND decided_at IS NOT NULL AND reviewed_by_user_id IS NOT NULL
            AND applied_at IS NULL AND applied_program_version IS NULL
            AND (rejection_reason IS NULL OR length(rejection_reason) <= 1000)
        )
        OR (
            status IN ('expired', 'superseded') AND reviewed_by_user_id IS NULL
            AND applied_at IS NULL AND applied_program_version IS NULL AND rejection_reason IS NULL
        )
    ),
    CONSTRAINT coach_recommendations_prompt_version_check CHECK (length(btrim(prompt_version)) BETWEEN 1 AND 100),
    CONSTRAINT coach_recommendations_version_check CHECK (version > 0),
    CONSTRAINT coach_recommendations_updated_at_check CHECK (updated_at >= created_at)
);

CREATE INDEX coach_recommendations_user_hash_idx ON coach_recommendations (user_id, proposal_hash);
CREATE INDEX coach_recommendations_source_user_idx ON coach_recommendations (source_message_id, user_id);
CREATE INDEX coach_recommendations_program_user_idx ON coach_recommendations (target_program_id, user_id);
CREATE INDEX coach_recommendations_reviewer_idx ON coach_recommendations (reviewed_by_user_id);
CREATE INDEX coach_recommendations_user_status_created_idx
    ON coach_recommendations (user_id, status, created_at DESC);
CREATE INDEX coach_recommendations_conversation_created_idx
    ON coach_recommendations (conversation_id, created_at);

CREATE TABLE weekly_reports (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    period_start_at timestamptz NOT NULL,
    period_end_at timestamptz NOT NULL,
    timezone_at_generation text NOT NULL,
    revision smallint NOT NULL,
    is_current boolean NOT NULL DEFAULT true,
    supersedes_report_id uuid,
    status text NOT NULL DEFAULT 'pending',
    metrics_schema_version smallint NOT NULL DEFAULT 1,
    metrics jsonb,
    input_data_through_at timestamptz,
    ai_insight_status text NOT NULL DEFAULT 'not_requested',
    ai_insight text,
    model text,
    prompt_version text,
    attempt_count smallint NOT NULL DEFAULT 0,
    processing_attempt_id uuid,
    processing_started_at timestamptz,
    lease_expires_at timestamptz,
    next_attempt_at timestamptz,
    retryable boolean NOT NULL DEFAULT false,
    generated_at timestamptz,
    error_code text,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT weekly_reports_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT weekly_reports_id_user_unique UNIQUE (id, user_id),
    CONSTRAINT weekly_reports_id_user_period_unique UNIQUE (id, user_id, period_start_at, period_end_at),
    CONSTRAINT weekly_reports_user_period_revision_unique
        UNIQUE (user_id, period_start_at, period_end_at, revision),
    CONSTRAINT weekly_reports_period_check CHECK (period_end_at > period_start_at),
    CONSTRAINT weekly_reports_timezone_check CHECK (
        timezone_at_generation = btrim(timezone_at_generation)
        AND length(timezone_at_generation) BETWEEN 1 AND 255
    ),
    CONSTRAINT weekly_reports_revision_check CHECK (revision > 0),
    CONSTRAINT weekly_reports_status_check CHECK (status IN ('pending', 'generating', 'ready', 'failed', 'stale')),
    CONSTRAINT weekly_reports_metrics_version_check CHECK (metrics_schema_version > 0),
    CONSTRAINT weekly_reports_metrics_object_check CHECK (metrics IS NULL OR jsonb_typeof(metrics) = 'object'),
    CONSTRAINT weekly_reports_ai_status_check CHECK (
        ai_insight_status IN ('not_requested', 'pending', 'ready', 'failed')
    ),
    CONSTRAINT weekly_reports_attempt_count_check CHECK (attempt_count BETWEEN 0 AND 100),
    CONSTRAINT weekly_reports_generation_lifecycle_check CHECK (
        (
            status = 'pending' AND metrics IS NULL AND input_data_through_at IS NULL AND generated_at IS NULL
            AND processing_attempt_id IS NULL AND processing_started_at IS NULL AND lease_expires_at IS NULL
            AND error_code IS NULL
        )
        OR (
            status = 'generating' AND metrics IS NULL AND input_data_through_at IS NULL AND generated_at IS NULL
            AND processing_attempt_id IS NOT NULL AND processing_started_at IS NOT NULL AND lease_expires_at IS NOT NULL
            AND lease_expires_at > processing_started_at AND error_code IS NULL
        )
        OR (
            status IN ('ready', 'stale') AND metrics IS NOT NULL AND input_data_through_at IS NOT NULL
            AND generated_at IS NOT NULL AND processing_attempt_id IS NULL AND processing_started_at IS NULL
            AND lease_expires_at IS NULL AND error_code IS NULL
        )
        OR (
            status = 'failed' AND processing_attempt_id IS NULL AND processing_started_at IS NULL
            AND lease_expires_at IS NULL AND error_code IS NOT NULL AND length(btrim(error_code)) BETWEEN 1 AND 100
        )
    ),
    CONSTRAINT weekly_reports_ai_lifecycle_check CHECK (
        (ai_insight_status = 'not_requested' AND ai_insight IS NULL AND model IS NULL AND prompt_version IS NULL)
        OR (ai_insight_status = 'pending' AND ai_insight IS NULL)
        OR (
            ai_insight_status = 'ready' AND ai_insight IS NOT NULL AND length(btrim(ai_insight)) > 0
            AND model IS NOT NULL AND length(btrim(model)) > 0
            AND prompt_version IS NOT NULL AND length(btrim(prompt_version)) > 0
        )
        OR (ai_insight_status = 'failed' AND ai_insight IS NULL)
    ),
    CONSTRAINT weekly_reports_version_check CHECK (version > 0),
    CONSTRAINT weekly_reports_updated_at_check CHECK (updated_at >= created_at)
);

ALTER TABLE weekly_reports
    ADD CONSTRAINT weekly_reports_supersedes_fk
    FOREIGN KEY (supersedes_report_id, user_id, period_start_at, period_end_at)
    REFERENCES weekly_reports (id, user_id, period_start_at, period_end_at)
    ON DELETE NO ACTION;

CREATE UNIQUE INDEX weekly_reports_current_period_uidx
    ON weekly_reports (user_id, period_start_at, period_end_at)
    WHERE is_current;
CREATE UNIQUE INDEX weekly_reports_unfinished_period_uidx
    ON weekly_reports (user_id, period_start_at, period_end_at)
    WHERE status IN ('pending', 'generating');
CREATE INDEX weekly_reports_supersedes_user_idx ON weekly_reports (supersedes_report_id, user_id);
CREATE INDEX weekly_reports_user_period_idx ON weekly_reports (user_id, period_start_at DESC);
CREATE INDEX weekly_reports_pending_work_idx ON weekly_reports (status, next_attempt_at, lease_expires_at);

CREATE TABLE idempotency_keys (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    method text NOT NULL,
    canonical_path text NOT NULL,
    request_hash bytea NOT NULL,
    state text NOT NULL,
    response_status smallint,
    response_headers jsonb,
    response_body jsonb,
    locked_until timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT idempotency_keys_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT idempotency_keys_scope_unique UNIQUE (user_id, method, canonical_path, idempotency_key),
    CONSTRAINT idempotency_keys_key_check CHECK (length(idempotency_key) BETWEEN 1 AND 255),
    CONSTRAINT idempotency_keys_method_check CHECK (
        method = upper(method) AND method IN ('POST', 'PUT', 'PATCH', 'DELETE')
    ),
    CONSTRAINT idempotency_keys_path_check CHECK (
        canonical_path = btrim(canonical_path) AND length(canonical_path) BETWEEN 1 AND 1000
    ),
    CONSTRAINT idempotency_keys_hash_check CHECK (octet_length(request_hash) = 32),
    CONSTRAINT idempotency_keys_state_check CHECK (state IN ('processing', 'completed')),
    CONSTRAINT idempotency_keys_response_headers_check CHECK (
        response_headers IS NULL OR jsonb_typeof(response_headers) = 'object'
    ),
    CONSTRAINT idempotency_keys_response_body_check CHECK (
        response_body IS NULL OR jsonb_typeof(response_body) = 'object'
    ),
    CONSTRAINT idempotency_keys_lifecycle_check CHECK (
        (
            state = 'processing' AND response_status IS NULL AND response_headers IS NULL
            AND response_body IS NULL AND locked_until IS NOT NULL
        )
        OR (
            state = 'completed' AND response_status IS NOT NULL AND response_status BETWEEN 100 AND 599
            AND response_body IS NOT NULL AND locked_until IS NULL
        )
    ),
    CONSTRAINT idempotency_keys_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT idempotency_keys_updated_at_check CHECK (updated_at >= created_at)
);

CREATE INDEX idempotency_keys_expires_idx ON idempotency_keys (expires_at);

CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    actor_user_id uuid,
    event_type text NOT NULL,
    resource_type text,
    resource_id uuid,
    request_id uuid,
    metadata jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT audit_events_actor_fk FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT audit_events_event_type_check CHECK (length(btrim(event_type)) BETWEEN 1 AND 200),
    CONSTRAINT audit_events_resource_type_check CHECK (
        resource_type IS NULL OR length(btrim(resource_type)) BETWEEN 1 AND 100
    ),
    CONSTRAINT audit_events_metadata_object_check CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX audit_events_actor_occurred_idx ON audit_events (actor_user_id, occurred_at DESC);
CREATE INDEX audit_events_resource_occurred_idx
    ON audit_events (resource_type, resource_id, occurred_at DESC);
CREATE INDEX audit_events_request_idx ON audit_events (request_id);
