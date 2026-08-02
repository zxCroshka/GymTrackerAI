SET TIME ZONE 'UTC';

DROP INDEX IF EXISTS exercises_catalog_filters_idx;

ALTER TABLE exercises
    DROP CONSTRAINT IF EXISTS exercises_tracking_presence_check,
    DROP CONSTRAINT IF EXISTS exercises_muscle_group_catalog_check,
    DROP CONSTRAINT IF EXISTS exercises_equipment_catalog_check,
    DROP CONSTRAINT IF EXISTS exercises_type_check,
    DROP COLUMN IF EXISTS tracks_distance,
    DROP COLUMN IF EXISTS tracks_time,
    DROP COLUMN IF EXISTS tracks_repetitions,
    DROP COLUMN IF EXISTS tracks_weight,
    DROP COLUMN IF EXISTS exercise_type;
