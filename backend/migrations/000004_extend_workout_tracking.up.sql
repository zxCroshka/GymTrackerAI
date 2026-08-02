SET TIME ZONE 'UTC';

ALTER TABLE workouts
    ADD COLUMN difficulty smallint,
    ADD COLUMN energy smallint,
    ADD COLUMN mood smallint,
    ADD COLUMN has_pain boolean,
    ADD COLUMN discomfort text,
    ADD CONSTRAINT workouts_difficulty_check CHECK (difficulty IS NULL OR difficulty BETWEEN 1 AND 10) NOT VALID,
    ADD CONSTRAINT workouts_energy_check CHECK (energy IS NULL OR energy BETWEEN 1 AND 10) NOT VALID,
    ADD CONSTRAINT workouts_mood_check CHECK (mood IS NULL OR mood BETWEEN 1 AND 10) NOT VALID,
    ADD CONSTRAINT workouts_discomfort_check CHECK (
        discomfort IS NULL OR (discomfort = btrim(discomfort) AND length(discomfort) BETWEEN 1 AND 4000)
    ) NOT VALID;

ALTER TABLE workouts
    VALIDATE CONSTRAINT workouts_difficulty_check,
    VALIDATE CONSTRAINT workouts_energy_check,
    VALIDATE CONSTRAINT workouts_mood_check,
    VALIDATE CONSTRAINT workouts_discomfort_check;

ALTER TABLE workout_exercises
    ADD COLUMN rest_seconds integer,
    ADD COLUMN tracks_weight boolean,
    ADD COLUMN tracks_repetitions boolean,
    ADD COLUMN tracks_time boolean,
    ADD COLUMN tracks_distance boolean,
    ADD CONSTRAINT workout_exercises_rest_check CHECK (
        rest_seconds IS NULL OR rest_seconds BETWEEN 0 AND 86400
    ) NOT VALID;

UPDATE workout_exercises AS workout_exercise
SET tracks_weight = exercise.tracks_weight,
    tracks_repetitions = exercise.tracks_repetitions,
    tracks_time = exercise.tracks_time,
    tracks_distance = exercise.tracks_distance
FROM exercises AS exercise
WHERE exercise.id = workout_exercise.exercise_id;

UPDATE workout_exercises AS workout_exercise
SET rest_seconds = prescription.rest_seconds
FROM program_day_exercises AS prescription
WHERE prescription.id = workout_exercise.source_program_day_exercise_id
  AND prescription.user_id = workout_exercise.user_id;

ALTER TABLE workout_exercises
    ALTER COLUMN tracks_weight SET NOT NULL,
    ALTER COLUMN tracks_repetitions SET NOT NULL,
    ALTER COLUMN tracks_time SET NOT NULL,
    ALTER COLUMN tracks_distance SET NOT NULL,
    ADD CONSTRAINT workout_exercises_tracking_presence_check CHECK (
        tracks_weight OR tracks_repetitions OR tracks_time OR tracks_distance
    ) NOT VALID;

ALTER TABLE workout_exercises
    VALIDATE CONSTRAINT workout_exercises_rest_check,
    VALIDATE CONSTRAINT workout_exercises_tracking_presence_check;

ALTER TABLE workout_sets
    DROP CONSTRAINT workout_sets_actual_check,
    ADD COLUMN duration_seconds integer,
    ADD COLUMN distance_meters numeric(12,3),
    ADD CONSTRAINT workout_sets_weight_check CHECK (weight_kg IS NULL OR weight_kg >= 0) NOT VALID,
    ADD CONSTRAINT workout_sets_reps_check CHECK (reps IS NULL OR reps >= 0) NOT VALID,
    ADD CONSTRAINT workout_sets_duration_check CHECK (
        duration_seconds IS NULL OR duration_seconds BETWEEN 0 AND 86400
    ) NOT VALID,
    ADD CONSTRAINT workout_sets_distance_check CHECK (
        distance_meters IS NULL OR distance_meters >= 0
    ) NOT VALID,
    ADD CONSTRAINT workout_sets_actual_check CHECK (
        (
            status = 'completed'
            AND completed_at IS NOT NULL
            AND num_nonnulls(weight_kg, reps, duration_seconds, distance_meters) > 0
        )
        OR (
            status IN ('planned', 'skipped')
            AND weight_kg IS NULL
            AND reps IS NULL
            AND rir IS NULL
            AND duration_seconds IS NULL
            AND distance_meters IS NULL
            AND completed_at IS NULL
        )
    ) NOT VALID;

ALTER TABLE workout_sets
    VALIDATE CONSTRAINT workout_sets_weight_check,
    VALIDATE CONSTRAINT workout_sets_reps_check,
    VALIDATE CONSTRAINT workout_sets_duration_check,
    VALIDATE CONSTRAINT workout_sets_distance_check,
    VALIDATE CONSTRAINT workout_sets_actual_check;

CREATE INDEX workouts_user_event_idx
    ON workouts (user_id, (COALESCE(started_at, scheduled_at, created_at)) DESC, id DESC);
CREATE INDEX workouts_user_status_event_idx
    ON workouts (user_id, status, (COALESCE(started_at, scheduled_at, created_at)) DESC, id DESC);
CREATE INDEX workouts_user_completed_idx
    ON workouts (user_id, completed_at DESC, id DESC)
    WHERE status = 'completed';
