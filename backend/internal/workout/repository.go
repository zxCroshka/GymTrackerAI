package workout

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rootColumns = `
	w.id,
	(SELECT day.program_id FROM program_days AS day WHERE day.id = w.source_program_day_id),
	w.source_program_day_id, w.source_program_version, w.name, w.status,
	w.scheduled_at, w.started_at, w.completed_at, w.cancelled_at,
	w.difficulty, w.energy, w.mood, w.notes, w.has_pain, w.discomfort,
	w.version, w.created_at, w.updated_at`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type aggregateQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *Repository) Get(ctx context.Context, actorID, workoutID string) (Workout, error) {
	return loadAggregate(ctx, r.pool, actorID, workoutID)
}

func loadAggregate(ctx context.Context, query aggregateQuery, actorID, workoutID string) (Workout, error) {
	value, err := scanRoot(query.QueryRow(ctx, "SELECT "+rootColumns+`
		FROM workouts AS w WHERE w.id = $1 AND w.user_id = $2`, workoutID, actorID))
	if err != nil {
		return Workout{}, err
	}
	rows, err := query.Query(ctx, `
		SELECT id, workout_id, exercise_id, source_program_day_exercise_id,
		       position, exercise_name_snapshot, notes, rest_seconds,
		       tracks_weight, tracks_repetitions, tracks_time, tracks_distance,
		       created_at, updated_at
		FROM workout_exercises
		WHERE workout_id = $1 AND user_id = $2
		ORDER BY position`, workoutID, actorID)
	if err != nil {
		return Workout{}, fmt.Errorf("list workout exercises: %w", err)
	}
	value.Exercises = []WorkoutExercise{}
	for rows.Next() {
		var item WorkoutExercise
		if err := rows.Scan(
			&item.ID, &item.WorkoutID, &item.ExerciseID, &item.SourceProgramDayExerciseID,
			&item.Position, &item.ExerciseNameSnapshot, &item.Comment, &item.RestSeconds,
			&item.TracksWeight, &item.TracksRepetitions, &item.TracksTime, &item.TracksDistance,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			rows.Close()
			return Workout{}, fmt.Errorf("scan workout exercise: %w", err)
		}
		item.CreatedAt, item.UpdatedAt = item.CreatedAt.UTC(), item.UpdatedAt.UTC()
		value.Exercises = append(value.Exercises, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Workout{}, fmt.Errorf("iterate workout exercises: %w", err)
	}
	rows.Close()
	for index := range value.Exercises {
		sets, err := query.Query(ctx, `
			SELECT id, workout_exercise_id, position, status,
			       target_weight_kg, target_reps_min, target_reps_max, target_rir,
			       weight_kg, reps, rir, set_type, set_type = 'warmup', set_type = 'failure',
			       duration_seconds, distance_meters, notes, completed_at,
			       created_at, updated_at
			FROM workout_sets
			WHERE workout_exercise_id = $1 AND user_id = $2
			ORDER BY position`, value.Exercises[index].ID, actorID)
		if err != nil {
			return Workout{}, fmt.Errorf("list workout sets: %w", err)
		}
		value.Exercises[index].Sets = []WorkoutSet{}
		for sets.Next() {
			item, err := scanSet(sets)
			if err != nil {
				sets.Close()
				return Workout{}, err
			}
			value.Exercises[index].Sets = append(value.Exercises[index].Sets, item)
		}
		if err := sets.Err(); err != nil {
			sets.Close()
			return Workout{}, fmt.Errorf("iterate workout sets: %w", err)
		}
		sets.Close()
	}
	calculateMetrics(&value)
	return value, nil
}

func (r *Repository) Active(ctx context.Context, actorID string) (*Workout, error) {
	var workoutID string
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM workouts WHERE user_id = $1 AND status = 'in_progress'`, actorID).Scan(&workoutID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find active workout: %w", err)
	}
	value, err := r.Get(ctx, actorID, workoutID)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *Repository) List(ctx context.Context, actorID string, filter ListFilter) (ListResult, error) {
	args := []any{actorID}
	conditions := []string{"w.user_id = $1"}
	conditions, args = appendFilterConditions(conditions, args, filter, false)
	if filter.Cursor != nil {
		args = append(args, filter.Cursor.EventAt.UTC(), filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf(
			"(COALESCE(w.started_at, w.scheduled_at, w.created_at), w.id) < ($%d, $%d::uuid)", len(args)-1, len(args)))
	}
	args = append(args, filter.Limit+1)
	statement := "SELECT " + rootColumns + `,
		stats.exercise_count, stats.set_count, stats.working_set_count,
		stats.volume_kg, stats.best_estimated_1rm_kg
		FROM workouts AS w
		LEFT JOIN LATERAL (
			SELECT count(DISTINCT exercise.id)::int AS exercise_count,
			       count(set.id) FILTER (WHERE set.status = 'completed')::int AS set_count,
			       count(set.id) FILTER (WHERE set.status = 'completed' AND set.set_type <> 'warmup')::int AS working_set_count,
			       COALESCE(sum(set.weight_kg * set.reps) FILTER (
					   WHERE set.status = 'completed' AND set.set_type <> 'warmup'
					     AND set.weight_kg IS NOT NULL AND set.reps IS NOT NULL), 0)::double precision AS volume_kg,
			       max(set.weight_kg * (1 + set.reps::numeric / 30)) FILTER (
					   WHERE set.status = 'completed' AND set.set_type <> 'warmup'
					     AND set.weight_kg > 0 AND set.reps BETWEEN 1 AND 15)::double precision AS best_estimated_1rm_kg
			FROM workout_exercises AS exercise
			LEFT JOIN workout_sets AS set ON set.workout_exercise_id = exercise.id
			WHERE exercise.workout_id = w.id AND exercise.user_id = w.user_id
		) AS stats ON true
		WHERE ` + strings.Join(conditions, " AND ") +
		fmt.Sprintf(" ORDER BY COALESCE(w.started_at, w.scheduled_at, w.created_at) DESC, w.id DESC LIMIT $%d", len(args))
	rows, err := r.pool.Query(ctx, statement, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("list workouts: %w", err)
	}
	defer rows.Close()
	result := ListResult{Items: []Workout{}}
	for rows.Next() {
		value, err := scanRootWithMetrics(rows)
		if err != nil {
			return ListResult{}, err
		}
		result.Items = append(result.Items, value)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, fmt.Errorf("iterate workouts: %w", err)
	}
	if len(result.Items) > filter.Limit {
		result.Items = result.Items[:filter.Limit]
		cursor, err := EncodeCursor(result.Items[len(result.Items)-1], filter)
		if err != nil {
			return ListResult{}, fmt.Errorf("encode workout cursor: %w", err)
		}
		result.NextCursor = &cursor
	}
	return result, nil
}

func appendFilterConditions(conditions []string, args []any, filter ListFilter, export bool) ([]string, []any) {
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("w.status = $%d", len(args)))
	}
	if filter.From != nil {
		args = append(args, filter.From.UTC())
		conditions = append(conditions, fmt.Sprintf("COALESCE(w.started_at, w.scheduled_at, w.created_at) >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, filter.To.UTC())
		conditions = append(conditions, fmt.Sprintf("COALESCE(w.started_at, w.scheduled_at, w.created_at) < $%d", len(args)))
	}
	if filter.ProgramID != "" {
		args = append(args, filter.ProgramID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM program_days AS filter_day
			WHERE filter_day.id = w.source_program_day_id AND filter_day.program_id = $%d
		)`, len(args)))
	}
	if filter.ExerciseID != "" && !export {
		args = append(args, filter.ExerciseID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM workout_exercises AS filter_exercise
			WHERE filter_exercise.workout_id = w.id AND filter_exercise.user_id = w.user_id
			  AND filter_exercise.exercise_id = $%d
		)`, len(args)))
	}
	return conditions, args
}

func (r *Repository) Lock(ctx context.Context, tx pgx.Tx, actorID, workoutID string) (Workout, error) {
	return scanRoot(tx.QueryRow(ctx, "SELECT "+rootColumns+`
		FROM workouts AS w WHERE w.id = $1 AND w.user_id = $2 FOR UPDATE`, workoutID, actorID))
}

func (r *Repository) LockByExercise(ctx context.Context, tx pgx.Tx, actorID, workoutExerciseID string) (Workout, WorkoutExercise, error) {
	root, err := scanRoot(tx.QueryRow(ctx, "SELECT "+rootColumns+`
		FROM workout_exercises AS item
		JOIN workouts AS w ON w.id = item.workout_id AND w.user_id = item.user_id
		WHERE item.id = $1 AND item.user_id = $2 FOR UPDATE OF w`, workoutExerciseID, actorID))
	if errors.Is(err, ErrNotFound) {
		return Workout{}, WorkoutExercise{}, ErrExerciseNotFound
	}
	if err != nil {
		return Workout{}, WorkoutExercise{}, err
	}
	item, err := r.GetExercise(ctx, tx, actorID, workoutExerciseID)
	return root, item, err
}

func (r *Repository) LockBySet(ctx context.Context, tx pgx.Tx, actorID, workoutSetID string) (Workout, WorkoutExercise, WorkoutSet, error) {
	root, err := scanRoot(tx.QueryRow(ctx, "SELECT "+rootColumns+`
		FROM workout_sets AS set
		JOIN workout_exercises AS item ON item.id = set.workout_exercise_id AND item.user_id = set.user_id
		JOIN workouts AS w ON w.id = item.workout_id AND w.user_id = item.user_id
		WHERE set.id = $1 AND set.user_id = $2 FOR UPDATE OF w`, workoutSetID, actorID))
	if errors.Is(err, ErrNotFound) {
		return Workout{}, WorkoutExercise{}, WorkoutSet{}, ErrSetNotFound
	}
	if err != nil {
		return Workout{}, WorkoutExercise{}, WorkoutSet{}, err
	}
	var item WorkoutExercise
	var set WorkoutSet
	err = tx.QueryRow(ctx, `
		SELECT item.id, item.workout_id, item.exercise_id, item.source_program_day_exercise_id,
		       item.position, item.exercise_name_snapshot, item.notes, item.rest_seconds,
		       item.tracks_weight, item.tracks_repetitions, item.tracks_time, item.tracks_distance,
		       item.created_at, item.updated_at,
		       set.id, set.workout_exercise_id, set.position, set.status,
		       set.target_weight_kg, set.target_reps_min, set.target_reps_max, set.target_rir,
		       set.weight_kg, set.reps, set.rir, set.set_type, set.set_type = 'warmup', set.set_type = 'failure',
		       set.duration_seconds, set.distance_meters, set.notes, set.completed_at,
		       set.created_at, set.updated_at
		FROM workout_sets AS set
		JOIN workout_exercises AS item ON item.id = set.workout_exercise_id AND item.user_id = set.user_id
		WHERE set.id = $1 AND set.user_id = $2`, workoutSetID, actorID).Scan(
		&item.ID, &item.WorkoutID, &item.ExerciseID, &item.SourceProgramDayExerciseID,
		&item.Position, &item.ExerciseNameSnapshot, &item.Comment, &item.RestSeconds,
		&item.TracksWeight, &item.TracksRepetitions, &item.TracksTime, &item.TracksDistance,
		&item.CreatedAt, &item.UpdatedAt,
		&set.ID, &set.WorkoutExerciseID, &set.SetNumber, &set.Status,
		&set.TargetWeightKG, &set.TargetRepsMin, &set.TargetRepsMax, &set.TargetRIR,
		&set.WeightKG, &set.Repetitions, &set.RIR, &set.SetType, &set.Warmup, &set.Failure,
		&set.DurationSeconds, &set.DistanceMeters, &set.Note, &set.PerformedAt,
		&set.CreatedAt, &set.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workout{}, WorkoutExercise{}, WorkoutSet{}, ErrSetNotFound
	}
	if err != nil {
		return Workout{}, WorkoutExercise{}, WorkoutSet{}, fmt.Errorf("load workout set for update: %w", err)
	}
	normalizeSetTimes(&set)
	return root, item, set, nil
}

func (r *Repository) GetExercise(ctx context.Context, query aggregateQuery, actorID, workoutExerciseID string) (WorkoutExercise, error) {
	var item WorkoutExercise
	err := query.QueryRow(ctx, `
		SELECT id, workout_id, exercise_id, source_program_day_exercise_id,
		       position, exercise_name_snapshot, notes, rest_seconds,
		       tracks_weight, tracks_repetitions, tracks_time, tracks_distance,
		       created_at, updated_at
		FROM workout_exercises WHERE id = $1 AND user_id = $2`, workoutExerciseID, actorID).Scan(
		&item.ID, &item.WorkoutID, &item.ExerciseID, &item.SourceProgramDayExerciseID,
		&item.Position, &item.ExerciseNameSnapshot, &item.Comment, &item.RestSeconds,
		&item.TracksWeight, &item.TracksRepetitions, &item.TracksTime, &item.TracksDistance,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkoutExercise{}, ErrExerciseNotFound
	}
	if err != nil {
		return WorkoutExercise{}, fmt.Errorf("get workout exercise: %w", err)
	}
	item.CreatedAt, item.UpdatedAt = item.CreatedAt.UTC(), item.UpdatedAt.UTC()
	return item, nil
}

func (r *Repository) InsertRoot(ctx context.Context, tx pgx.Tx, actorID string, value Workout) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO workouts (
			id, user_id, source_program_day_id, source_program_version, name, status,
			scheduled_at, started_at, difficulty, energy, mood, notes, has_pain,
			discomfort, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 1, $15, $15
		)`, value.ID, actorID, value.SourceProgramDayID, value.SourceProgramVersion, value.Name,
		value.Status, value.ScheduledAt, value.StartedAt, value.Difficulty, value.Energy,
		value.Mood, value.Comment, value.HasPain, value.Discomfort, value.CreatedAt.UTC())
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "workouts_one_in_progress_per_user_uidx" {
			return ErrActiveExists
		}
		return fmt.Errorf("insert workout: %w", err)
	}
	return nil
}

func (r *Repository) UpdateRoot(ctx context.Context, tx pgx.Tx, actorID string, value Workout, expectedVersion int64, now time.Time) error {
	result, err := tx.Exec(ctx, `
		UPDATE workouts SET
			name = $4, status = $5, scheduled_at = $6, started_at = $7,
			completed_at = $8, cancelled_at = $9, difficulty = $10, energy = $11,
			mood = $12, notes = $13, has_pain = $14, discomfort = $15,
			version = version + 1, updated_at = $16
		WHERE id = $1 AND user_id = $2 AND version = $3`, value.ID, actorID, expectedVersion,
		value.Name, value.Status, value.ScheduledAt, value.StartedAt, value.CompletedAt,
		value.CancelledAt, value.Difficulty, value.Energy, value.Mood, value.Comment,
		value.HasPain, value.Discomfort, now.UTC())
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "workouts_one_in_progress_per_user_uidx" {
			return ErrActiveExists
		}
		return fmt.Errorf("update workout: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) Touch(ctx context.Context, tx pgx.Tx, actorID, workoutID string, expectedVersion int64, now time.Time) error {
	result, err := tx.Exec(ctx, `
		UPDATE workouts SET version = version + 1, updated_at = $4
		WHERE id = $1 AND user_id = $2 AND version = $3`, workoutID, actorID, expectedVersion, now.UTC())
	if err != nil {
		return fmt.Errorf("advance workout version: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, tx pgx.Tx, actorID, workoutID string) error {
	result, err := tx.Exec(ctx, `DELETE FROM workouts WHERE id = $1 AND user_id = $2`, workoutID, actorID)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ExerciseCount(ctx context.Context, tx pgx.Tx, actorID, workoutID string) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM workout_exercises WHERE workout_id = $1 AND user_id = $2`, workoutID, actorID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count workout exercises: %w", err)
	}
	return count, nil
}

func (r *Repository) SetCount(ctx context.Context, tx pgx.Tx, actorID, workoutExerciseID string) (int, error) {
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM workout_sets WHERE workout_exercise_id = $1 AND user_id = $2`, workoutExerciseID, actorID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count workout sets: %w", err)
	}
	return count, nil
}

func (r *Repository) InsertExercise(ctx context.Context, tx pgx.Tx, actorID string, item WorkoutExercise, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO workout_exercises (
			id, user_id, workout_id, exercise_id, source_program_day_exercise_id,
			position, exercise_name_snapshot, notes, rest_seconds, tracks_weight,
			tracks_repetitions, tracks_time, tracks_distance, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14
		)`, item.ID, actorID, item.WorkoutID, item.ExerciseID, item.SourceProgramDayExerciseID,
		item.Position, item.ExerciseNameSnapshot, item.Comment, item.RestSeconds,
		item.TracksWeight, item.TracksRepetitions, item.TracksTime, item.TracksDistance, now.UTC())
	if err != nil {
		return fmt.Errorf("insert workout exercise: %w", err)
	}
	return nil
}

func (r *Repository) InsertPlannedSet(ctx context.Context, tx pgx.Tx, actorID string, set WorkoutSet, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO workout_sets (
			id, user_id, workout_exercise_id, position, set_type, status,
			target_weight_kg, target_reps_min, target_reps_max, target_rir,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, 'working', 'planned', $5, $6, $7, $8, $9, $9)`,
		set.ID, actorID, set.WorkoutExerciseID, set.SetNumber, set.TargetWeightKG,
		set.TargetRepsMin, set.TargetRepsMax, set.TargetRIR, now.UTC())
	if err != nil {
		return fmt.Errorf("insert planned workout set: %w", err)
	}
	return nil
}

func (r *Repository) InsertCompletedSet(ctx context.Context, tx pgx.Tx, actorID string, set WorkoutSet, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO workout_sets (
			id, user_id, workout_exercise_id, position, set_type, status,
			weight_kg, reps, rir, duration_seconds, distance_meters, notes,
			completed_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, 'completed', $6, $7, $8, $9, $10, $11, $12, $13, $13)`,
		set.ID, actorID, set.WorkoutExerciseID, set.SetNumber, setType(set.Warmup, set.Failure),
		set.WeightKG, set.Repetitions, set.RIR, set.DurationSeconds, set.DistanceMeters,
		set.Note, set.PerformedAt, now.UTC())
	if err != nil {
		return fmt.Errorf("insert workout set: %w", err)
	}
	return nil
}

func (r *Repository) UpdateExercise(ctx context.Context, tx pgx.Tx, actorID string, item WorkoutExercise, now time.Time) error {
	result, err := tx.Exec(ctx, `
		UPDATE workout_exercises SET position = $3, notes = $4, updated_at = $5
		WHERE id = $1 AND user_id = $2`, item.ID, actorID, item.Position, item.Comment, now.UTC())
	if err != nil {
		return fmt.Errorf("update workout exercise: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrExerciseNotFound
	}
	return nil
}

func (r *Repository) UpdateSet(ctx context.Context, tx pgx.Tx, actorID string, set WorkoutSet, now time.Time) error {
	result, err := tx.Exec(ctx, `
		UPDATE workout_sets SET
			position = $3, set_type = $4, status = $5, weight_kg = $6, reps = $7,
			rir = $8, duration_seconds = $9, distance_meters = $10, notes = $11,
			completed_at = $12, updated_at = $13
		WHERE id = $1 AND user_id = $2`, set.ID, actorID, set.SetNumber,
		set.SetType, set.Status, set.WeightKG, set.Repetitions,
		set.RIR, set.DurationSeconds, set.DistanceMeters, set.Note, set.PerformedAt, now.UTC())
	if err != nil {
		return fmt.Errorf("update workout set: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrSetNotFound
	}
	return nil
}

func (r *Repository) ShiftExercisesForInsert(ctx context.Context, tx pgx.Tx, actorID, workoutID string, position int16) error {
	return shiftForInsert(ctx, tx, "workout_exercises", "workout_id", actorID, workoutID, position)
}

func (r *Repository) ShiftSetsForInsert(ctx context.Context, tx pgx.Tx, actorID, workoutExerciseID string, position int16) error {
	return shiftForInsert(ctx, tx, "workout_sets", "workout_exercise_id", actorID, workoutExerciseID, position)
}

func shiftForInsert(ctx context.Context, tx pgx.Tx, table, parentColumn, actorID, parentID string, position int16) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position + 1000
		WHERE %s = $1 AND user_id = $2 AND position >= $3`, table, parentColumn), parentID, actorID, position); err != nil {
		return fmt.Errorf("make position gap: %w", err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position - 999
		WHERE %s = $1 AND user_id = $2 AND position >= 1000`, table, parentColumn), parentID, actorID); err != nil {
		return fmt.Errorf("close temporary position range: %w", err)
	}
	return nil
}

func (r *Repository) MoveExercise(ctx context.Context, tx pgx.Tx, actorID, workoutID, itemID string, oldPosition, newPosition int16) error {
	return movePosition(ctx, tx, "workout_exercises", "workout_id", actorID, workoutID, itemID, oldPosition, newPosition)
}

func (r *Repository) MoveSet(ctx context.Context, tx pgx.Tx, actorID, workoutExerciseID, setID string, oldPosition, newPosition int16) error {
	return movePosition(ctx, tx, "workout_sets", "workout_exercise_id", actorID, workoutExerciseID, setID, oldPosition, newPosition)
}

func movePosition(ctx context.Context, tx pgx.Tx, table, parentColumn, actorID, parentID, itemID string, oldPosition, newPosition int16) error {
	if oldPosition == newPosition {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = 30000 WHERE id = $1 AND user_id = $2`, table), itemID, actorID); err != nil {
		return fmt.Errorf("park position: %w", err)
	}
	if newPosition < oldPosition {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position + 1000
			WHERE %s = $1 AND user_id = $2 AND position BETWEEN $3 AND $4`, table, parentColumn), parentID, actorID, newPosition, oldPosition-1); err != nil {
			return fmt.Errorf("move position range: %w", err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position - 999
			WHERE %s = $1 AND user_id = $2 AND position BETWEEN $3 AND $4`, table, parentColumn), parentID, actorID, newPosition+1000, oldPosition-1+1000); err != nil {
			return fmt.Errorf("normalize moved position range: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position + 1000
			WHERE %s = $1 AND user_id = $2 AND position BETWEEN $3 AND $4`, table, parentColumn), parentID, actorID, oldPosition+1, newPosition); err != nil {
			return fmt.Errorf("move position range: %w", err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position - 1001
			WHERE %s = $1 AND user_id = $2 AND position BETWEEN $3 AND $4`, table, parentColumn), parentID, actorID, oldPosition+1+1000, newPosition+1000); err != nil {
			return fmt.Errorf("normalize moved position range: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = $3 WHERE id = $1 AND user_id = $2`, table), itemID, actorID, newPosition); err != nil {
		return fmt.Errorf("place moved position: %w", err)
	}
	return nil
}

func (r *Repository) DeleteExercise(ctx context.Context, tx pgx.Tx, actorID string, item WorkoutExercise) error {
	if _, err := tx.Exec(ctx, `DELETE FROM workout_exercises WHERE id = $1 AND user_id = $2`, item.ID, actorID); err != nil {
		return fmt.Errorf("delete workout exercise: %w", err)
	}
	return closePositionGap(ctx, tx, "workout_exercises", "workout_id", actorID, item.WorkoutID, item.Position)
}

func (r *Repository) DeleteSet(ctx context.Context, tx pgx.Tx, actorID string, item WorkoutSet) error {
	if _, err := tx.Exec(ctx, `DELETE FROM workout_sets WHERE id = $1 AND user_id = $2`, item.ID, actorID); err != nil {
		return fmt.Errorf("delete workout set: %w", err)
	}
	return closePositionGap(ctx, tx, "workout_sets", "workout_exercise_id", actorID, item.WorkoutExerciseID, item.SetNumber)
}

func closePositionGap(ctx context.Context, tx pgx.Tx, table, parentColumn, actorID, parentID string, oldPosition int16) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position + 1000
		WHERE %s = $1 AND user_id = $2 AND position > $3`, table, parentColumn), parentID, actorID, oldPosition); err != nil {
		return fmt.Errorf("stage position gap close: %w", err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET position = position - 1001
		WHERE %s = $1 AND user_id = $2 AND position >= 1000`, table, parentColumn), parentID, actorID); err != nil {
		return fmt.Errorf("close position gap: %w", err)
	}
	return nil
}

func (r *Repository) MarkRemainingSetsSkipped(ctx context.Context, tx pgx.Tx, actorID, workoutID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE workout_sets AS set SET status = 'skipped', updated_at = $3
		FROM workout_exercises AS item
		WHERE set.workout_exercise_id = item.id AND set.user_id = $1
		  AND item.workout_id = $2 AND item.user_id = $1 AND set.status = 'planned'`,
		actorID, workoutID, now.UTC())
	if err != nil {
		return fmt.Errorf("skip unperformed workout sets: %w", err)
	}
	return nil
}

func (r *Repository) PerformedRange(ctx context.Context, tx pgx.Tx, actorID, workoutID string) (*time.Time, *time.Time, error) {
	var minimum, maximum *time.Time
	err := tx.QueryRow(ctx, `
		SELECT min(set.completed_at), max(set.completed_at)
		FROM workout_sets AS set
		JOIN workout_exercises AS item ON item.id = set.workout_exercise_id AND item.user_id = set.user_id
		WHERE item.workout_id = $1 AND item.user_id = $2 AND set.status = 'completed'`, workoutID, actorID).Scan(&minimum, &maximum)
	if err != nil {
		return nil, nil, fmt.Errorf("read performed set range: %w", err)
	}
	return utcTime(minimum), utcTime(maximum), nil
}

func (r *Repository) PreviousResult(ctx context.Context, actorID, anchorID string) (*PreviousResult, error) {
	var exerciseID, currentWorkoutID string
	var currentEvent time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT item.exercise_id, workout.id,
		       COALESCE(workout.started_at, workout.scheduled_at, workout.created_at)
		FROM workout_exercises AS item
		JOIN workouts AS workout ON workout.id = item.workout_id AND workout.user_id = item.user_id
		WHERE item.id = $1 AND item.user_id = $2`, anchorID, actorID).Scan(&exerciseID, &currentWorkoutID, &currentEvent)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrExerciseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load previous-result anchor: %w", err)
	}
	var result PreviousResult
	err = r.pool.QueryRow(ctx, `
		SELECT item.exercise_id, item.exercise_name_snapshot, workout.id, item.id,
		       workout.name, workout.started_at, workout.completed_at
		FROM workout_exercises AS item
		JOIN workouts AS workout ON workout.id = item.workout_id AND workout.user_id = item.user_id
		WHERE item.user_id = $1 AND item.exercise_id = $2 AND workout.status = 'completed'
		  AND (COALESCE(workout.started_at, workout.scheduled_at, workout.created_at), workout.id) < ($3, $4::uuid)
		ORDER BY COALESCE(workout.started_at, workout.scheduled_at, workout.created_at) DESC,
		         workout.id DESC, item.position
		LIMIT 1`, actorID, exerciseID, currentEvent.UTC(), currentWorkoutID).Scan(
		&result.ExerciseID, &result.ExerciseName, &result.SourceWorkoutID,
		&result.SourceWorkoutExerciseID, &result.WorkoutName, &result.StartedAt, &result.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find previous exercise result: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, workout_exercise_id, position, status,
		       target_weight_kg, target_reps_min, target_reps_max, target_rir,
		       weight_kg, reps, rir, set_type, set_type = 'warmup', set_type = 'failure',
		       duration_seconds, distance_meters, notes, completed_at, created_at, updated_at
		FROM workout_sets
		WHERE workout_exercise_id = $1 AND user_id = $2 AND status = 'completed'
		ORDER BY position`, result.SourceWorkoutExerciseID, actorID)
	if err != nil {
		return nil, fmt.Errorf("list previous result sets: %w", err)
	}
	defer rows.Close()
	result.Sets = []WorkoutSet{}
	for rows.Next() {
		set, err := scanSet(rows)
		if err != nil {
			return nil, err
		}
		set.VolumeKG = Volume(set.WeightKG, set.Repetitions, set.Warmup)
		set.Estimated1RMKG = Estimated1RM(set.WeightKG, set.Repetitions, set.Warmup)
		result.Sets = append(result.Sets, set)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate previous result sets: %w", err)
	}
	result.StartedAt, result.CompletedAt = result.StartedAt.UTC(), result.CompletedAt.UTC()
	return &result, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanRoot(row rowScanner) (Workout, error) {
	var value Workout
	err := row.Scan(
		&value.ID, &value.SourceProgramID, &value.SourceProgramDayID, &value.SourceProgramVersion,
		&value.Name, &value.Status, &value.ScheduledAt, &value.StartedAt, &value.CompletedAt,
		&value.CancelledAt, &value.Difficulty, &value.Energy, &value.Mood, &value.Comment,
		&value.HasPain, &value.Discomfort, &value.Version, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workout{}, ErrNotFound
	}
	if err != nil {
		return Workout{}, fmt.Errorf("scan workout: %w", err)
	}
	normalizeWorkoutTimes(&value)
	value.CalculationVersion = CalculationVersion
	return value, nil
}

func scanRootWithMetrics(row rowScanner) (Workout, error) {
	var value Workout
	err := row.Scan(
		&value.ID, &value.SourceProgramID, &value.SourceProgramDayID, &value.SourceProgramVersion,
		&value.Name, &value.Status, &value.ScheduledAt, &value.StartedAt, &value.CompletedAt,
		&value.CancelledAt, &value.Difficulty, &value.Energy, &value.Mood, &value.Comment,
		&value.HasPain, &value.Discomfort, &value.Version, &value.CreatedAt, &value.UpdatedAt,
		&value.ExerciseCount, &value.SetCount, &value.WorkingSetCount,
		&value.VolumeKG, &value.BestEstimated1RMKG,
	)
	if err != nil {
		return Workout{}, fmt.Errorf("scan workout summary: %w", err)
	}
	normalizeWorkoutTimes(&value)
	value.VolumeKG = roundMetric(value.VolumeKG)
	if value.BestEstimated1RMKG != nil {
		rounded := roundMetric(*value.BestEstimated1RMKG)
		value.BestEstimated1RMKG = &rounded
	}
	value.CalculationVersion = CalculationVersion
	return value, nil
}

func scanSet(row rowScanner) (WorkoutSet, error) {
	var value WorkoutSet
	err := row.Scan(
		&value.ID, &value.WorkoutExerciseID, &value.SetNumber, &value.Status,
		&value.TargetWeightKG, &value.TargetRepsMin, &value.TargetRepsMax, &value.TargetRIR,
		&value.WeightKG, &value.Repetitions, &value.RIR, &value.SetType, &value.Warmup, &value.Failure,
		&value.DurationSeconds, &value.DistanceMeters, &value.Note, &value.PerformedAt,
		&value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return WorkoutSet{}, fmt.Errorf("scan workout set: %w", err)
	}
	normalizeSetTimes(&value)
	return value, nil
}

func normalizeWorkoutTimes(value *Workout) {
	value.ScheduledAt = utcTime(value.ScheduledAt)
	value.StartedAt = utcTime(value.StartedAt)
	value.CompletedAt = utcTime(value.CompletedAt)
	value.CancelledAt = utcTime(value.CancelledAt)
	value.CreatedAt, value.UpdatedAt = value.CreatedAt.UTC(), value.UpdatedAt.UTC()
	value.EventAt = workoutEventAt(*value)
}

func normalizeSetTimes(value *WorkoutSet) {
	value.PerformedAt = utcTime(value.PerformedAt)
	value.CreatedAt, value.UpdatedAt = value.CreatedAt.UTC(), value.UpdatedAt.UTC()
}

func setType(warmup, failure bool) string {
	if warmup {
		return "warmup"
	}
	if failure {
		return "failure"
	}
	return "working"
}
