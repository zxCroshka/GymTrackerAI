package exercise

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

const exerciseColumns = `
	id, owner_user_id, name, description, instructions, primary_muscle_group,
	exercise_type, equipment, movement_pattern, is_unilateral,
	tracks_weight, tracks_repetitions, tracks_time, tracks_distance,
	version, archived_at, created_at, updated_at`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, actorID string, filter ListFilter) (ListResult, error) {
	args := []any{actorID}
	conditions := []string{"(owner_user_id IS NULL OR owner_user_id = $1)"}
	switch filter.Scope {
	case "", "all":
	case "system":
		conditions = append(conditions, "owner_user_id IS NULL")
	case "mine":
		conditions = append(conditions, "owner_user_id = $1")
	default:
		return ListResult{}, ErrValidation
	}
	if filter.IncludeArchived {
		conditions = append(conditions, "(archived_at IS NULL OR owner_user_id = $1)")
	} else {
		conditions = append(conditions, "archived_at IS NULL")
	}
	addTextFilter := func(column, value string) {
		if value == "" {
			return
		}
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if filter.Query != "" {
		args = append(args, strings.ToLower(filter.Query))
		conditions = append(conditions, fmt.Sprintf("strpos(lower(name), $%d) > 0", len(args)))
	}
	addTextFilter("primary_muscle_group", filter.MuscleGroup)
	addTextFilter("exercise_type", filter.ExerciseType)
	addTextFilter("equipment", filter.Equipment)
	addBoolFilter := func(column string, value *bool) {
		if value == nil {
			return
		}
		args = append(args, *value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addBoolFilter("tracks_weight", filter.TracksWeight)
	addBoolFilter("tracks_repetitions", filter.TracksRepetitions)
	addBoolFilter("tracks_time", filter.TracksTime)
	addBoolFilter("tracks_distance", filter.TracksDistance)
	if filter.Cursor != nil {
		args = append(args, filter.Cursor.Name, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf(
			"(lower(name), id) > ($%d, $%d::uuid)", len(args)-1, len(args),
		))
	}
	args = append(args, filter.Limit+1)
	statement := "SELECT " + exerciseColumns + " FROM exercises WHERE " +
		strings.Join(conditions, " AND ") +
		fmt.Sprintf(" ORDER BY lower(name), id LIMIT $%d", len(args))
	rows, err := r.pool.Query(ctx, statement, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("list exercises: %w", err)
	}
	defer rows.Close()
	result := ListResult{Items: []Exercise{}}
	for rows.Next() {
		value, err := scanExercise(rows)
		if err != nil {
			return ListResult{}, err
		}
		result.Items = append(result.Items, value)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, fmt.Errorf("iterate exercises: %w", err)
	}
	if len(result.Items) > filter.Limit {
		result.Items = result.Items[:filter.Limit]
		cursor, err := EncodeCursor(result.Items[len(result.Items)-1])
		if err != nil {
			return ListResult{}, fmt.Errorf("encode exercise cursor: %w", err)
		}
		result.NextCursor = &cursor
	}
	return result, nil
}

func (r *Repository) GetVisible(ctx context.Context, actorID, exerciseID string) (Exercise, error) {
	row := r.pool.QueryRow(ctx, "SELECT "+exerciseColumns+`
		FROM exercises
		WHERE id = $1 AND (owner_user_id IS NULL OR owner_user_id = $2)`,
		exerciseID, actorID)
	return scanExercise(row)
}

func (r *Repository) LockVisible(ctx context.Context, tx pgx.Tx, actorID, exerciseID string) (Exercise, error) {
	row := tx.QueryRow(ctx, "SELECT "+exerciseColumns+`
		FROM exercises
		WHERE id = $1 AND (owner_user_id IS NULL OR owner_user_id = $2)
		FOR UPDATE`, exerciseID, actorID)
	return scanExercise(row)
}

func (r *Repository) Insert(ctx context.Context, tx pgx.Tx, value Exercise) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO exercises (
			id, owner_user_id, name, description, instructions, primary_muscle_group,
			exercise_type, equipment, movement_pattern, is_unilateral,
			tracks_weight, tracks_repetitions, tracks_time, tracks_distance,
			version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, 1, $15, $15
		)`,
		value.ID, value.OwnerUserID, value.Name, value.Description, value.Instructions,
		value.PrimaryMuscleGroup, value.ExerciseType, value.Equipment, value.MovementPattern,
		value.IsUnilateral, value.TracksWeight, value.TracksRepetitions, value.TracksTime,
		value.TracksDistance, value.CreatedAt.UTC())
	if err == nil {
		return nil
	}
	if nameConflict(err) {
		return ErrNameConflict
	}
	return fmt.Errorf("insert exercise: %w", err)
}

func (r *Repository) Update(ctx context.Context, tx pgx.Tx, actorID string, value Exercise, expectedVersion int64, now time.Time) error {
	tag, err := tx.Exec(ctx, `
		UPDATE exercises SET
			name = $4, description = $5, instructions = $6, primary_muscle_group = $7,
			exercise_type = $8, equipment = $9, movement_pattern = $10,
			is_unilateral = $11, tracks_weight = $12, tracks_repetitions = $13,
			tracks_time = $14, tracks_distance = $15,
			version = version + 1, updated_at = $16
		WHERE id = $1 AND owner_user_id = $2 AND version = $3 AND archived_at IS NULL`,
		value.ID, actorID, expectedVersion, value.Name, value.Description, value.Instructions,
		value.PrimaryMuscleGroup, value.ExerciseType, value.Equipment, value.MovementPattern,
		value.IsUnilateral, value.TracksWeight, value.TracksRepetitions, value.TracksTime,
		value.TracksDistance, now.UTC())
	if err != nil {
		if nameConflict(err) {
			return ErrNameConflict
		}
		return fmt.Errorf("update exercise: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) Archive(ctx context.Context, tx pgx.Tx, actorID, exerciseID string, expectedVersion int64, now time.Time) error {
	tag, err := tx.Exec(ctx, `
		UPDATE exercises
		SET archived_at = $4, version = version + 1, updated_at = $4
		WHERE id = $1 AND owner_user_id = $2 AND version = $3 AND archived_at IS NULL`,
		exerciseID, actorID, expectedVersion, now.UTC())
	if err != nil {
		return fmt.Errorf("archive exercise: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) IsUsable(ctx context.Context, tx pgx.Tx, actorID, exerciseID string) (bool, error) {
	var usable bool
	err := tx.QueryRow(ctx, `
		SELECT true FROM exercises
		WHERE id = $1 AND archived_at IS NULL
		AND (owner_user_id IS NULL OR owner_user_id = $2)
		FOR KEY SHARE`, exerciseID, actorID).Scan(&usable)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check exercise visibility: %w", err)
	}
	return usable, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanExercise(row rowScanner) (Exercise, error) {
	var value Exercise
	err := row.Scan(
		&value.ID, &value.OwnerUserID, &value.Name, &value.Description, &value.Instructions,
		&value.PrimaryMuscleGroup, &value.ExerciseType, &value.Equipment, &value.MovementPattern,
		&value.IsUnilateral, &value.TracksWeight, &value.TracksRepetitions, &value.TracksTime,
		&value.TracksDistance, &value.Version, &value.ArchivedAt, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Exercise{}, ErrNotFound
	}
	if err != nil {
		return Exercise{}, fmt.Errorf("scan exercise: %w", err)
	}
	value.CreatedAt = value.CreatedAt.UTC()
	value.UpdatedAt = value.UpdatedAt.UTC()
	if value.ArchivedAt != nil {
		archived := value.ArchivedAt.UTC()
		value.ArchivedAt = &archived
	}
	return value, nil
}

func nameConflict(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505" &&
		(pgError.ConstraintName == "exercises_owner_active_name_uidx" ||
			pgError.ConstraintName == "exercises_global_name_uidx")
}
