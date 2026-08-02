SET TIME ZONE 'UTC';

ALTER TABLE exercises
    ADD COLUMN exercise_type text NOT NULL DEFAULT 'strength',
    ADD COLUMN tracks_weight boolean NOT NULL DEFAULT true,
    ADD COLUMN tracks_repetitions boolean NOT NULL DEFAULT true,
    ADD COLUMN tracks_time boolean NOT NULL DEFAULT false,
    ADD COLUMN tracks_distance boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT exercises_type_check CHECK (
        exercise_type IN ('strength', 'cardio', 'stretching', 'bodyweight', 'isometric')
    ),
    ADD CONSTRAINT exercises_equipment_catalog_check CHECK (
        equipment IS NULL OR equipment IN (
            'barbell', 'dumbbell', 'machine', 'cable', 'pullup_bar',
            'parallel_bars', 'bodyweight', 'other'
        )
    ),
    ADD CONSTRAINT exercises_muscle_group_catalog_check CHECK (
        primary_muscle_group IS NULL OR primary_muscle_group IN (
            'chest', 'back', 'quadriceps', 'hamstrings', 'glutes',
            'posterior_chain', 'shoulders', 'biceps', 'triceps',
            'calves', 'core', 'full_body', 'cardio', 'other'
        )
    ),
    ADD CONSTRAINT exercises_tracking_presence_check CHECK (
        tracks_weight OR tracks_repetitions OR tracks_time OR tracks_distance
    );

CREATE INDEX exercises_catalog_filters_idx
    ON exercises (exercise_type, equipment, primary_muscle_group, id);
