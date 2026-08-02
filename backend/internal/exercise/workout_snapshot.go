package exercise

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// WorkoutSnapshot is stable exercise metadata copied into a workout.
type WorkoutSnapshot struct {
	ID                string
	Name              string
	TracksWeight      bool
	TracksRepetitions bool
	TracksTime        bool
	TracksDistance    bool
}

func (s *Service) SnapshotForWorkout(ctx context.Context, tx pgx.Tx, actorID, exerciseID string, allowArchived bool) (WorkoutSnapshot, error) {
	return s.repository.SnapshotForWorkout(ctx, tx, actorID, exerciseID, allowArchived)
}

func (r *Repository) SnapshotForWorkout(ctx context.Context, tx pgx.Tx, actorID, exerciseID string, allowArchived bool) (WorkoutSnapshot, error) {
	var snapshot WorkoutSnapshot
	err := tx.QueryRow(ctx, `
		SELECT id, name, tracks_weight, tracks_repetitions, tracks_time, tracks_distance
		FROM exercises
		WHERE id = $1
		  AND (owner_user_id IS NULL OR owner_user_id = $2)
		  AND ($3 OR archived_at IS NULL)
		FOR SHARE`, exerciseID, actorID, allowArchived).Scan(
		&snapshot.ID, &snapshot.Name, &snapshot.TracksWeight,
		&snapshot.TracksRepetitions, &snapshot.TracksTime, &snapshot.TracksDistance,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkoutSnapshot{}, ErrNotFound
	}
	if err != nil {
		return WorkoutSnapshot{}, fmt.Errorf("snapshot exercise for workout: %w", err)
	}
	return snapshot, nil
}
