package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type systemExerciseSeed struct {
	id                 string
	name               string
	primaryMuscleGroup string
	equipment          string
	movementPattern    string
}

var systemExerciseSeeds = []systemExerciseSeed{
	{id: "00000000-0000-4000-8000-000000000001", name: "Back Squat", primaryMuscleGroup: "quadriceps", equipment: "barbell", movementPattern: "squat"},
	{id: "00000000-0000-4000-8000-000000000002", name: "Bench Press", primaryMuscleGroup: "chest", equipment: "barbell", movementPattern: "horizontal_push"},
	{id: "00000000-0000-4000-8000-000000000003", name: "Deadlift", primaryMuscleGroup: "posterior_chain", equipment: "barbell", movementPattern: "hinge"},
}

// SeedSystemExercises idempotently installs the small baseline system
// catalogue. A larger reviewed catalogue can use the same mechanism later.
func SeedSystemExercises(ctx context.Context, beginner Beginner) (int64, error) {
	var affected int64
	err := WithinTransaction(ctx, beginner, pgx.TxOptions{}, func(tx pgx.Tx) error {
		for _, exercise := range systemExerciseSeeds {
			tag, err := tx.Exec(ctx, `
                INSERT INTO exercises (
                    id, owner_user_id, name, primary_muscle_group, equipment,
                    movement_pattern, is_unilateral, version
                )
                VALUES ($1, NULL, $2, $3, $4, $5, false, 1)
                ON CONFLICT (id) DO UPDATE SET
                    name = EXCLUDED.name,
                    primary_muscle_group = EXCLUDED.primary_muscle_group,
                    equipment = EXCLUDED.equipment,
                    movement_pattern = EXCLUDED.movement_pattern,
                    is_unilateral = EXCLUDED.is_unilateral,
					version = exercises.version + 1,
					archived_at = NULL,
					updated_at = CURRENT_TIMESTAMP
				WHERE exercises.owner_user_id IS NULL
				AND ROW(
					exercises.name,
					exercises.primary_muscle_group,
					exercises.equipment,
					exercises.movement_pattern,
					exercises.is_unilateral,
					exercises.archived_at
				) IS DISTINCT FROM ROW(
					EXCLUDED.name,
					EXCLUDED.primary_muscle_group,
					EXCLUDED.equipment,
					EXCLUDED.movement_pattern,
					EXCLUDED.is_unilateral,
					NULL::timestamptz
				)
			`, exercise.id, exercise.name, exercise.primaryMuscleGroup, exercise.equipment, exercise.movementPattern)
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
