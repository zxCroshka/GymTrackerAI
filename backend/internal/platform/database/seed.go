package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type systemExerciseSeed struct {
	id, name, muscleGroup, exerciseType, equipment string
	tracksWeight, tracksRepetitions, tracksTime    bool
	tracksDistance                                 bool
}

var systemExerciseSeeds = []systemExerciseSeed{
	{"00000000-0000-4000-8000-000000000001", "Приседания со штангой", "quadriceps", "strength", "barbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000002", "Жим штанги лёжа", "chest", "strength", "barbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000003", "Становая тяга", "posterior_chain", "strength", "barbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000004", "Жим Hammer Strength", "chest", "strength", "machine", true, true, false, false},
	{"00000000-0000-4000-8000-000000000005", "Подтягивания", "back", "bodyweight", "pullup_bar", false, true, false, false},
	{"00000000-0000-4000-8000-000000000006", "Подтягивания с весом", "back", "strength", "pullup_bar", true, true, false, false},
	{"00000000-0000-4000-8000-000000000007", "Тяга штанги в наклоне", "back", "strength", "barbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000008", "Румынская тяга", "hamstrings", "strength", "barbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000009", "Жим ногами", "quadriceps", "strength", "machine", true, true, false, false},
	{"00000000-0000-4000-8000-000000000010", "Разгибание ног", "quadriceps", "strength", "machine", true, true, false, false},
	{"00000000-0000-4000-8000-000000000011", "Сгибание ног", "hamstrings", "strength", "machine", true, true, false, false},
	{"00000000-0000-4000-8000-000000000012", "Жим гантелей", "chest", "strength", "dumbbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000013", "Подъём гантелей на бицепс", "biceps", "strength", "dumbbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000014", "Разгибание рук на блоке", "triceps", "strength", "cable", true, true, false, false},
	{"00000000-0000-4000-8000-000000000015", "Махи в стороны", "shoulders", "strength", "dumbbell", true, true, false, false},
	{"00000000-0000-4000-8000-000000000016", "Планка", "core", "isometric", "bodyweight", false, false, true, false},
	{"00000000-0000-4000-8000-000000000017", "Бег", "cardio", "cardio", "other", false, false, true, true},
	{"00000000-0000-4000-8000-000000000018", "Ходьба", "cardio", "cardio", "other", false, false, true, true},
	{"00000000-0000-4000-8000-000000000019", "Настольный теннис", "full_body", "cardio", "other", false, false, true, false},
}

// SeedSystemExercises idempotently installs the reviewed system catalogue.
func SeedSystemExercises(ctx context.Context, beginner Beginner) (int64, error) {
	var affected int64
	err := WithinTransaction(ctx, beginner, pgx.TxOptions{}, func(tx pgx.Tx) error {
		for _, exercise := range systemExerciseSeeds {
			tag, err := tx.Exec(ctx, `
				INSERT INTO exercises (
					id, owner_user_id, name, primary_muscle_group, exercise_type,
					equipment, tracks_weight, tracks_repetitions, tracks_time,
					tracks_distance, is_unilateral, version
				) VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, $9, false, 1)
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					primary_muscle_group = EXCLUDED.primary_muscle_group,
					exercise_type = EXCLUDED.exercise_type,
					equipment = EXCLUDED.equipment,
					tracks_weight = EXCLUDED.tracks_weight,
					tracks_repetitions = EXCLUDED.tracks_repetitions,
					tracks_time = EXCLUDED.tracks_time,
					tracks_distance = EXCLUDED.tracks_distance,
					is_unilateral = EXCLUDED.is_unilateral,
					version = exercises.version + 1,
					archived_at = NULL,
					updated_at = CURRENT_TIMESTAMP
				WHERE exercises.owner_user_id IS NULL
				AND ROW(
					exercises.name, exercises.primary_muscle_group,
					exercises.exercise_type, exercises.equipment,
					exercises.tracks_weight, exercises.tracks_repetitions,
					exercises.tracks_time, exercises.tracks_distance,
					exercises.is_unilateral, exercises.archived_at
				) IS DISTINCT FROM ROW(
					EXCLUDED.name, EXCLUDED.primary_muscle_group,
					EXCLUDED.exercise_type, EXCLUDED.equipment,
					EXCLUDED.tracks_weight, EXCLUDED.tracks_repetitions,
					EXCLUDED.tracks_time, EXCLUDED.tracks_distance,
					EXCLUDED.is_unilateral, NULL::timestamptz
				)`,
				exercise.id, exercise.name, exercise.muscleGroup, exercise.exerciseType,
				exercise.equipment, exercise.tracksWeight, exercise.tracksRepetitions,
				exercise.tracksTime, exercise.tracksDistance)
			if err != nil {
				return fmt.Errorf("seed system exercise %s: %w", exercise.id, err)
			}
			affected += tag.RowsAffected()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}
