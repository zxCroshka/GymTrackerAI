package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrVersionConflict = errors.New("profile version conflict")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateDefaultProfile(ctx context.Context, tx pgx.Tx, userID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_profiles (user_id, timezone, preferred_unit_system, created_at, updated_at)
		VALUES ($1, 'UTC', 'metric', $2, $2)`, userID, now.UTC())
	if err != nil {
		return fmt.Errorf("create default user profile: %w", err)
	}
	return nil
}

type profileQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *Repository) Get(ctx context.Context, userID string) (databaseProfile, error) {
	return loadProfile(ctx, r.pool, userID, false)
}

func (r *Repository) Lock(ctx context.Context, tx pgx.Tx, userID string) (databaseProfile, error) {
	return loadProfile(ctx, tx, userID, true)
}

func loadProfile(ctx context.Context, query profileQuery, userID string, lock bool) (databaseProfile, error) {
	statement := `
		SELECT user_id, display_name, sex, birth_date, height_cm, goal, experience_level,
		       training_frequency, timezone, preferred_unit_system, sleep_hours_average,
		       version, created_at, updated_at
		FROM user_profiles WHERE user_id = $1`
	if lock {
		statement += " FOR UPDATE"
	}
	var profile databaseProfile
	err := query.QueryRow(ctx, statement, userID).Scan(
		&profile.UserID, &profile.Name, &profile.Sex, &profile.BirthDate, &profile.HeightCM,
		&profile.Goal, &profile.ExperienceLevel, &profile.TrainingFrequency, &profile.Timezone,
		&profile.UnitSystem, &profile.SleepHoursAverage, &profile.Version, &profile.CreatedAt, &profile.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return databaseProfile{}, ErrProfileNotFound
	}
	if err != nil {
		return databaseProfile{}, fmt.Errorf("get user profile: %w", err)
	}
	rows, err := query.Query(ctx, `
		SELECT content FROM user_profile_notes
		WHERE user_id = $1 ORDER BY position`, userID)
	if err != nil {
		return databaseProfile{}, fmt.Errorf("get profile notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var note string
		if err := rows.Scan(&note); err != nil {
			return databaseProfile{}, fmt.Errorf("scan profile note: %w", err)
		}
		profile.Notes = append(profile.Notes, note)
	}
	if err := rows.Err(); err != nil {
		return databaseProfile{}, fmt.Errorf("iterate profile notes: %w", err)
	}
	return profile, nil
}

func (r *Repository) Update(ctx context.Context, tx pgx.Tx, profile databaseProfile, expectedVersion int64, now time.Time) (int64, error) {
	var version int64
	err := tx.QueryRow(ctx, `
		UPDATE user_profiles SET
			display_name = $3, sex = $4, birth_date = $5::date, height_cm = $6,
			goal = $7, experience_level = $8, training_frequency = $9,
			timezone = $10, preferred_unit_system = $11, sleep_hours_average = $12,
			version = version + 1, updated_at = $13
		WHERE user_id = $1 AND version = $2
		RETURNING version`,
		profile.UserID, expectedVersion, profile.Name, profile.Sex, dateParameter(profile.BirthDate),
		profile.HeightCM, profile.Goal, profile.ExperienceLevel, profile.TrainingFrequency,
		profile.Timezone, profile.UnitSystem, profile.SleepHoursAverage, now.UTC(),
	).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrVersionConflict
	}
	if err != nil {
		return 0, fmt.Errorf("update user profile: %w", err)
	}
	return version, nil
}

func dateParameter(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.DateOnly)
}

func (r *Repository) ReplaceNotes(ctx context.Context, tx pgx.Tx, userID string, notes []string, now time.Time) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_profile_notes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete profile notes: %w", err)
	}
	for index, note := range notes {
		noteID, err := id.UUID()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_profile_notes (id, user_id, position, content, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)`,
			noteID, userID, index+1, note, now.UTC()); err != nil {
			return fmt.Errorf("insert profile note: %w", err)
		}
	}
	return nil
}
