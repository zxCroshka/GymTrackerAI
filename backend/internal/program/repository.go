package program

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const programColumns = `
	id, name, description, goal, status, version,
	activated_at, inactivated_at, archived_at, created_at, updated_at`

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

func (r *Repository) List(ctx context.Context, actorID string, filter ListFilter) (ListResult, error) {
	args := []any{actorID}
	conditions := []string{"user_id = $1"}
	if filter.IncludeArchived {
		if filter.Status != "" {
			args = append(args, filter.Status)
			conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
		}
	} else {
		conditions = append(conditions, "status <> 'archived'")
		if filter.Status != "" {
			args = append(args, filter.Status)
			conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
		}
	}
	if filter.Cursor != nil {
		args = append(args, filter.Cursor.UpdatedAt.UTC(), filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf(
			"(updated_at, id) < ($%d, $%d::uuid)", len(args)-1, len(args),
		))
	}
	args = append(args, filter.Limit+1)
	statement := "SELECT " + programColumns + " FROM programs WHERE " +
		strings.Join(conditions, " AND ") +
		fmt.Sprintf(" ORDER BY updated_at DESC, id DESC LIMIT $%d", len(args))
	rows, err := r.pool.Query(ctx, statement, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("list programs: %w", err)
	}
	defer rows.Close()
	result := ListResult{Items: []Program{}}
	for rows.Next() {
		value, err := scanProgram(rows)
		if err != nil {
			return ListResult{}, err
		}
		result.Items = append(result.Items, value)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, fmt.Errorf("iterate programs: %w", err)
	}
	if len(result.Items) > filter.Limit {
		result.Items = result.Items[:filter.Limit]
		cursor, err := EncodeCursor(result.Items[len(result.Items)-1])
		if err != nil {
			return ListResult{}, fmt.Errorf("encode program cursor: %w", err)
		}
		result.NextCursor = &cursor
	}
	return result, nil
}

func (r *Repository) Get(ctx context.Context, actorID, programID string) (Program, error) {
	return loadAggregate(ctx, r.pool, actorID, programID, false)
}

func (r *Repository) Lock(ctx context.Context, tx pgx.Tx, actorID, programID string) (Program, error) {
	return loadAggregate(ctx, tx, actorID, programID, true)
}

func loadAggregate(ctx context.Context, query aggregateQuery, actorID, programID string, lock bool) (Program, error) {
	statement := "SELECT " + programColumns + `
		FROM programs WHERE id = $1 AND user_id = $2`
	if lock {
		statement += " FOR UPDATE"
	}
	value, err := scanProgram(query.QueryRow(ctx, statement, programID, actorID))
	if err != nil {
		return Program{}, err
	}
	days, err := query.Query(ctx, `
		SELECT id, position, name, notes, created_at, updated_at
		FROM program_days
		WHERE program_id = $1 AND user_id = $2 AND archived_at IS NULL
		ORDER BY position`, programID, actorID)
	if err != nil {
		return Program{}, fmt.Errorf("list program days: %w", err)
	}
	defer days.Close()
	value.Days = []ProgramDay{}
	for days.Next() {
		var day ProgramDay
		if err := days.Scan(&day.ID, &day.Position, &day.Name, &day.Notes, &day.CreatedAt, &day.UpdatedAt); err != nil {
			return Program{}, fmt.Errorf("scan program day: %w", err)
		}
		day.CreatedAt, day.UpdatedAt = day.CreatedAt.UTC(), day.UpdatedAt.UTC()
		value.Days = append(value.Days, day)
	}
	if err := days.Err(); err != nil {
		return Program{}, fmt.Errorf("iterate program days: %w", err)
	}
	for index := range value.Days {
		items, err := query.Query(ctx, `
			SELECT id, exercise_id, position, target_sets, target_reps_min,
			       target_reps_max, target_rir, rest_seconds, notes, created_at, updated_at
			FROM program_day_exercises
			WHERE program_day_id = $1 AND user_id = $2 AND archived_at IS NULL
			ORDER BY position`, value.Days[index].ID, actorID)
		if err != nil {
			return Program{}, fmt.Errorf("list program day exercises: %w", err)
		}
		value.Days[index].Exercises = []ProgramDayExercise{}
		for items.Next() {
			var item ProgramDayExercise
			if err := items.Scan(
				&item.ID, &item.ExerciseID, &item.Position, &item.WorkingSets,
				&item.TargetRepsMin, &item.TargetRepsMax, &item.TargetRIR,
				&item.RestSeconds, &item.Notes, &item.CreatedAt, &item.UpdatedAt,
			); err != nil {
				items.Close()
				return Program{}, fmt.Errorf("scan program day exercise: %w", err)
			}
			item.CreatedAt, item.UpdatedAt = item.CreatedAt.UTC(), item.UpdatedAt.UTC()
			value.Days[index].Exercises = append(value.Days[index].Exercises, item)
		}
		if err := items.Err(); err != nil {
			items.Close()
			return Program{}, fmt.Errorf("iterate program day exercises: %w", err)
		}
		items.Close()
	}
	return value, nil
}

func (r *Repository) InsertRoot(ctx context.Context, tx pgx.Tx, actorID string, value Program) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO programs (
			id, user_id, name, description, goal, status, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, 'draft', 1, $6, $6)`,
		value.ID, actorID, value.Name, value.Description, value.Goal, value.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert program: %w", err)
	}
	return nil
}

func (r *Repository) InsertTree(ctx context.Context, tx pgx.Tx, actorID, programID string, days []ProgramDay, now time.Time) error {
	for _, day := range days {
		if _, err := tx.Exec(ctx, `
			INSERT INTO program_days (
				id, user_id, program_id, position, name, notes, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
			day.ID, actorID, programID, day.Position, day.Name, day.Notes, now.UTC()); err != nil {
			return fmt.Errorf("insert program day: %w", err)
		}
		for _, item := range day.Exercises {
			if _, err := tx.Exec(ctx, `
				INSERT INTO program_day_exercises (
					id, user_id, program_day_id, exercise_id, position, target_sets,
					target_reps_min, target_reps_max, target_rir, rest_seconds, notes,
					created_at, updated_at
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12
				)`,
				item.ID, actorID, day.ID, item.ExerciseID, item.Position, item.WorkingSets,
				item.TargetRepsMin, item.TargetRepsMax, item.TargetRIR, item.RestSeconds,
				item.Notes, now.UTC()); err != nil {
				return fmt.Errorf("insert program day exercise: %w", err)
			}
		}
	}
	return nil
}

func (r *Repository) ArchiveTree(ctx context.Context, tx pgx.Tx, actorID, programID string, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE program_day_exercises AS item
		SET archived_at = $3, updated_at = $3
		FROM program_days AS day
		WHERE item.program_day_id = day.id AND item.user_id = $1
		AND day.program_id = $2 AND day.user_id = $1
		AND item.archived_at IS NULL`, actorID, programID, now.UTC()); err != nil {
		return fmt.Errorf("archive program day exercises: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE program_days SET archived_at = $3, updated_at = $3
		WHERE user_id = $1 AND program_id = $2 AND archived_at IS NULL`,
		actorID, programID, now.UTC()); err != nil {
		return fmt.Errorf("archive program days: %w", err)
	}
	return nil
}

func (r *Repository) UpdateRoot(ctx context.Context, tx pgx.Tx, actorID string, value Program, expectedVersion int64, now time.Time) error {
	tag, err := tx.Exec(ctx, `
		UPDATE programs SET
			name = $4, description = $5, goal = $6,
			version = version + 1, updated_at = $7
		WHERE id = $1 AND user_id = $2 AND version = $3 AND status <> 'archived'`,
		value.ID, actorID, expectedVersion, value.Name, value.Description,
		value.Goal, now.UTC())
	if err != nil {
		return fmt.Errorf("update program: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) Archive(ctx context.Context, tx pgx.Tx, actorID, programID string, expectedVersion int64, now time.Time) error {
	tag, err := tx.Exec(ctx, `
		UPDATE programs SET
			status = 'archived', archived_at = $4,
			inactivated_at = CASE WHEN status = 'active' THEN $4 ELSE inactivated_at END,
			version = version + 1, updated_at = $4
		WHERE id = $1 AND user_id = $2 AND version = $3 AND status <> 'archived'`,
		programID, actorID, expectedVersion, now.UTC())
	if err != nil {
		return fmt.Errorf("archive program: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) LockRootsForActivation(ctx context.Context, tx pgx.Tx, actorID string) ([]Program, error) {
	rows, err := tx.Query(ctx, "SELECT "+programColumns+`
		FROM programs WHERE user_id = $1 ORDER BY id FOR UPDATE`, actorID)
	if err != nil {
		return nil, fmt.Errorf("lock programs for activation: %w", err)
	}
	defer rows.Close()
	var programs []Program
	for rows.Next() {
		value, err := scanProgram(rows)
		if err != nil {
			return nil, err
		}
		programs = append(programs, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locked programs: %w", err)
	}
	return programs, nil
}

func (r *Repository) DeactivateCurrent(ctx context.Context, tx pgx.Tx, actorID, targetID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE programs SET
			status = 'inactive', inactivated_at = $3,
			version = version + 1, updated_at = $3
		WHERE user_id = $1 AND id <> $2 AND status = 'active'`,
		actorID, targetID, now.UTC())
	if err != nil {
		return fmt.Errorf("deactivate current program: %w", err)
	}
	return nil
}

func (r *Repository) ActivateTarget(ctx context.Context, tx pgx.Tx, actorID, programID string, expectedVersion int64, now time.Time) error {
	tag, err := tx.Exec(ctx, `
		UPDATE programs SET
			status = 'active', activated_at = $4, inactivated_at = NULL,
			version = version + 1, updated_at = $4
		WHERE id = $1 AND user_id = $2 AND version = $3 AND status <> 'archived'`,
		programID, actorID, expectedVersion, now.UTC())
	if err != nil {
		return fmt.Errorf("activate program: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanProgram(row rowScanner) (Program, error) {
	var value Program
	err := row.Scan(
		&value.ID, &value.Name, &value.Description, &value.Goal, &value.Status,
		&value.Version, &value.ActivatedAt, &value.InactivatedAt, &value.ArchivedAt,
		&value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Program{}, ErrNotFound
	}
	if err != nil {
		return Program{}, fmt.Errorf("scan program: %w", err)
	}
	value.CreatedAt, value.UpdatedAt = value.CreatedAt.UTC(), value.UpdatedAt.UTC()
	value.ActivatedAt = utcPointer(value.ActivatedAt)
	value.InactivatedAt = utcPointer(value.InactivatedAt)
	value.ArchivedAt = utcPointer(value.ArchivedAt)
	return value, nil
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
