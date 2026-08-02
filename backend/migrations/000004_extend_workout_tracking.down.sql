SET TIME ZONE 'UTC';

-- Prove that every row can be represented by the preceding schema before
-- dropping any columns. Validation failure aborts the migration without
-- rewriting workout history.
LOCK TABLE workout_exercises, exercises, program_day_exercises IN SHARE MODE;

DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM workout_exercises AS workout_exercise
        JOIN exercises AS exercise
          ON exercise.id = workout_exercise.exercise_id
        LEFT JOIN program_day_exercises AS prescription
          ON prescription.id = workout_exercise.source_program_day_exercise_id
         AND prescription.user_id = workout_exercise.user_id
        WHERE workout_exercise.tracks_weight IS DISTINCT FROM exercise.tracks_weight
           OR workout_exercise.tracks_repetitions IS DISTINCT FROM exercise.tracks_repetitions
           OR workout_exercise.tracks_time IS DISTINCT FROM exercise.tracks_time
           OR workout_exercise.tracks_distance IS DISTINCT FROM exercise.tracks_distance
           OR (
                workout_exercise.source_program_day_exercise_id IS NULL
                AND workout_exercise.rest_seconds IS NOT NULL
           )
           OR (
                workout_exercise.source_program_day_exercise_id IS NOT NULL
                AND workout_exercise.rest_seconds IS DISTINCT FROM prescription.rest_seconds
           )
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = 'check_violation',
            MESSAGE = '000004 rollback refused: workout exercise snapshots are not representable by 000003';
    END IF;
END;
$migration$;

ALTER TABLE workouts
    ADD CONSTRAINT workouts_rollback_representation_check CHECK (
        difficulty IS NULL
        AND energy IS NULL
        AND mood IS NULL
        AND has_pain IS NULL
        AND discomfort IS NULL
    ) NOT VALID;

ALTER TABLE workout_sets
    ADD CONSTRAINT workout_sets_rollback_metrics_check CHECK (
        duration_seconds IS NULL AND distance_meters IS NULL
    ) NOT VALID,
    ADD CONSTRAINT workout_sets_rollback_actual_check CHECK (
        (
            status = 'completed'
            AND weight_kg IS NOT NULL
            AND weight_kg >= 0
            AND reps IS NOT NULL
            AND reps >= 0
            AND completed_at IS NOT NULL
        )
        OR (
            status IN ('planned', 'skipped')
            AND weight_kg IS NULL
            AND reps IS NULL
            AND rir IS NULL
            AND completed_at IS NULL
        )
    ) NOT VALID;

ALTER TABLE workouts
    VALIDATE CONSTRAINT workouts_rollback_representation_check;

ALTER TABLE workout_sets
    VALIDATE CONSTRAINT workout_sets_rollback_metrics_check,
    VALIDATE CONSTRAINT workout_sets_rollback_actual_check;

DROP INDEX IF EXISTS workouts_user_completed_idx;
DROP INDEX IF EXISTS workouts_user_status_event_idx;
DROP INDEX IF EXISTS workouts_user_event_idx;

ALTER TABLE workout_sets
    DROP CONSTRAINT workout_sets_actual_check,
    DROP CONSTRAINT workout_sets_rollback_metrics_check,
    DROP CONSTRAINT workout_sets_distance_check,
    DROP CONSTRAINT workout_sets_duration_check,
    DROP CONSTRAINT workout_sets_reps_check,
    DROP CONSTRAINT workout_sets_weight_check,
    DROP COLUMN distance_meters,
    DROP COLUMN duration_seconds;

ALTER TABLE workout_sets
    RENAME CONSTRAINT workout_sets_rollback_actual_check TO workout_sets_actual_check;

ALTER TABLE workout_exercises
    DROP CONSTRAINT IF EXISTS workout_exercises_tracking_presence_check,
    DROP CONSTRAINT IF EXISTS workout_exercises_rest_check,
    DROP COLUMN IF EXISTS tracks_distance,
    DROP COLUMN IF EXISTS tracks_time,
    DROP COLUMN IF EXISTS tracks_repetitions,
    DROP COLUMN IF EXISTS tracks_weight,
    DROP COLUMN IF EXISTS rest_seconds;

ALTER TABLE workouts
    DROP CONSTRAINT workouts_rollback_representation_check,
    DROP CONSTRAINT workouts_discomfort_check,
    DROP CONSTRAINT workouts_mood_check,
    DROP CONSTRAINT workouts_energy_check,
    DROP CONSTRAINT workouts_difficulty_check,
    DROP COLUMN discomfort,
    DROP COLUMN has_pain,
    DROP COLUMN mood,
    DROP COLUMN energy,
    DROP COLUMN difficulty;
