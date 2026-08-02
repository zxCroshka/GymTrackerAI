package program

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// WorkoutDaySnapshot is the immutable prescription view consumed by workout.
// It deliberately contains no workout persistence types.
type WorkoutDaySnapshot struct {
	ProgramID      string
	ProgramVersion int64
	DayID          string
	DayName        string
	Items          []WorkoutPrescription
}

type WorkoutPrescription struct {
	SourceItemID  string
	ExerciseID    string
	Position      int16
	WorkingSets   int16
	TargetRepsMin *int16
	TargetRepsMax *int16
	TargetRIR     *float64
	RestSeconds   *int32
	Notes         *string
}

func (s *Service) SnapshotActiveDay(ctx context.Context, tx pgx.Tx, actorID, dayID string) (WorkoutDaySnapshot, error) {
	return s.repository.SnapshotActiveDay(ctx, tx, actorID, dayID)
}

func (r *Repository) SnapshotActiveDay(ctx context.Context, tx pgx.Tx, actorID, dayID string) (WorkoutDaySnapshot, error) {
	var snapshot WorkoutDaySnapshot
	err := tx.QueryRow(ctx, `
		SELECT program.id, program.version, day.id, day.name
		FROM program_days AS day
		JOIN programs AS program
		  ON program.id = day.program_id AND program.user_id = day.user_id
		WHERE day.id = $1 AND day.user_id = $2
		  AND day.archived_at IS NULL
		  AND program.status = 'active'
		FOR SHARE OF program, day`, dayID, actorID).Scan(
		&snapshot.ProgramID, &snapshot.ProgramVersion, &snapshot.DayID, &snapshot.DayName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkoutDaySnapshot{}, ErrNotFound
	}
	if err != nil {
		return WorkoutDaySnapshot{}, fmt.Errorf("snapshot active program day: %w", err)
	}
	rows, err := tx.Query(ctx, `
		SELECT id, exercise_id, position, target_sets, target_reps_min,
		       target_reps_max, target_rir, rest_seconds, notes
		FROM program_day_exercises
		WHERE program_day_id = $1 AND user_id = $2 AND archived_at IS NULL
		ORDER BY position
		FOR SHARE`, dayID, actorID)
	if err != nil {
		return WorkoutDaySnapshot{}, fmt.Errorf("snapshot program prescriptions: %w", err)
	}
	defer rows.Close()
	snapshot.Items = []WorkoutPrescription{}
	for rows.Next() {
		var item WorkoutPrescription
		if err := rows.Scan(
			&item.SourceItemID, &item.ExerciseID, &item.Position, &item.WorkingSets,
			&item.TargetRepsMin, &item.TargetRepsMax, &item.TargetRIR,
			&item.RestSeconds, &item.Notes,
		); err != nil {
			return WorkoutDaySnapshot{}, fmt.Errorf("scan program prescription snapshot: %w", err)
		}
		snapshot.Items = append(snapshot.Items, item)
	}
	if err := rows.Err(); err != nil {
		return WorkoutDaySnapshot{}, fmt.Errorf("iterate program prescription snapshot: %w", err)
	}
	return snapshot, nil
}
