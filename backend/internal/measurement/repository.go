package measurement

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InitialMeasurement struct {
	ID, UserID               string
	MeasuredAt               time.Time
	WeightKG                 *float64
	ChestCM, WaistCM, HipsCM *float64
	NeckCM, BicepsCM         *float64
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool ...*pgxpool.Pool) *Repository {
	value := &Repository{}
	if len(pool) > 0 {
		value.pool = pool[0]
	}
	return value
}

type measurementQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

const bodyColumns = `
	id, measured_at, weight_kg, chest_cm, waist_cm, hips_cm, neck_cm,
	left_upper_arm_cm, right_upper_arm_cm, left_thigh_cm, right_thigh_cm,
	body_fat_percent, notes, source, version, created_at, updated_at`

func (r *Repository) InsertInitial(ctx context.Context, tx pgx.Tx, value InitialMeasurement) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO body_measurements (
			id, user_id, measured_at, weight_kg, chest_cm, waist_cm, hips_cm,
			neck_cm, left_upper_arm_cm, right_upper_arm_cm, source, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, 'import', $3, $3)`,
		value.ID, value.UserID, value.MeasuredAt.UTC(), value.WeightKG, value.ChestCM,
		value.WaistCM, value.HipsCM, value.NeckCM, value.BicepsCM)
	if err != nil {
		return fmt.Errorf("insert initial body measurement: %w", err)
	}
	return nil
}

func (r *Repository) InsertBody(ctx context.Context, tx pgx.Tx, actorID string, value BodyMeasurement) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO body_measurements (
			id, user_id, measured_at, weight_kg, chest_cm, waist_cm, hips_cm, neck_cm,
			left_upper_arm_cm, right_upper_arm_cm, left_thigh_cm, right_thigh_cm,
			body_fat_percent, notes, source, version, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'manual',1,$15,$15)`,
		value.ID, actorID, value.MeasuredAt.UTC(), value.WeightKG, value.ChestCM,
		value.WaistCM, value.HipsCM, value.NeckCM, value.LeftUpperArmCM,
		value.RightUpperArmCM, value.LeftThighCM, value.RightThighCM,
		value.BodyFatPercent, value.Notes, value.CreatedAt.UTC())
	if err != nil {
		return bodyWriteError("insert body measurement", err)
	}
	return nil
}

func (r *Repository) GetBody(ctx context.Context, actorID, measurementID string) (BodyMeasurement, error) {
	return scanBody(r.pool.QueryRow(ctx, `SELECT `+bodyColumns+`
		FROM body_measurements WHERE id=$1 AND user_id=$2`, measurementID, actorID))
}

func (r *Repository) LockBody(ctx context.Context, tx pgx.Tx, actorID, measurementID string) (BodyMeasurement, error) {
	return scanBody(tx.QueryRow(ctx, `SELECT `+bodyColumns+`
		FROM body_measurements WHERE id=$1 AND user_id=$2 FOR UPDATE`, measurementID, actorID))
}

func (r *Repository) UpdateBody(ctx context.Context, tx pgx.Tx, actorID string, value BodyMeasurement, expectedVersion int64, now time.Time) error {
	result, err := tx.Exec(ctx, `
		UPDATE body_measurements SET
			measured_at=$4, weight_kg=$5, chest_cm=$6, waist_cm=$7, hips_cm=$8,
			neck_cm=$9, left_upper_arm_cm=$10, right_upper_arm_cm=$11,
			left_thigh_cm=$12, right_thigh_cm=$13, body_fat_percent=$14,
			notes=$15, version=version+1, updated_at=$16
		WHERE id=$1 AND user_id=$2 AND version=$3`, value.ID, actorID, expectedVersion,
		value.MeasuredAt.UTC(), value.WeightKG, value.ChestCM, value.WaistCM,
		value.HipsCM, value.NeckCM, value.LeftUpperArmCM, value.RightUpperArmCM,
		value.LeftThighCM, value.RightThighCM, value.BodyFatPercent, value.Notes, now.UTC())
	if err != nil {
		return bodyWriteError("update body measurement", err)
	}
	if result.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) DeleteBody(ctx context.Context, tx pgx.Tx, actorID, measurementID string, expectedVersion int64) error {
	result, err := tx.Exec(ctx, `DELETE FROM body_measurements WHERE id=$1 AND user_id=$2 AND version=$3`, measurementID, actorID, expectedVersion)
	if err != nil {
		return fmt.Errorf("delete body measurement: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) ListBody(ctx context.Context, actorID string, filter ListFilter) (BodyListResult, error) {
	args := []any{actorID}
	where := `user_id=$1`
	if filter.From != nil {
		args = append(args, filter.From.UTC())
		where += fmt.Sprintf(" AND measured_at >= $%d", len(args))
	}
	if filter.To != nil {
		args = append(args, filter.To.UTC())
		where += fmt.Sprintf(" AND measured_at < $%d", len(args))
	}
	if filter.Cursor != nil {
		at, _ := time.Parse(time.RFC3339Nano, filter.Cursor.At)
		args = append(args, at.UTC(), filter.Cursor.ID)
		where += fmt.Sprintf(" AND (measured_at,id) < ($%d,$%d::uuid)", len(args)-1, len(args))
	}
	args = append(args, filter.Limit+1)
	rows, err := r.pool.Query(ctx, `SELECT `+bodyColumns+` FROM body_measurements WHERE `+where+
		fmt.Sprintf(" ORDER BY measured_at DESC,id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return BodyListResult{}, fmt.Errorf("list body measurements: %w", err)
	}
	defer rows.Close()
	result := BodyListResult{Items: []BodyMeasurement{}}
	for rows.Next() {
		value, err := scanBody(rows)
		if err != nil {
			return BodyListResult{}, err
		}
		result.Items = append(result.Items, value)
	}
	if err := rows.Err(); err != nil {
		return BodyListResult{}, fmt.Errorf("iterate body measurements: %w", err)
	}
	if len(result.Items) > filter.Limit {
		result.Items = result.Items[:filter.Limit]
		cursor, err := encodeCursor(result.Items[len(result.Items)-1].MeasuredAt, result.Items[len(result.Items)-1].ID)
		if err != nil {
			return BodyListResult{}, err
		}
		result.NextCursor = &cursor
	}
	return result, nil
}

func (r *Repository) InsertWellness(ctx context.Context, tx pgx.Tx, actorID string, value WellnessEntry) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO daily_wellness (
			id,user_id,observed_at,day_start_at,timezone_at_entry,sleep_minutes,
			sleep_quality,energy_level,steps,calories_kcal,protein_g,fat_g,carbs_g,
			notes,version,created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,1,$15,$15)`,
		value.ID, actorID, value.ObservedAt.UTC(), value.DayStartAt.UTC(), value.TimezoneAtEntry,
		value.SleepMinutes, value.SleepQuality, value.Energy, value.Steps, value.CaloriesKcal,
		value.ProteinG, value.FatG, value.CarbsG, value.Notes, value.CreatedAt.UTC())
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "daily_wellness_user_day_unique" {
			return ErrWellnessExists
		}
		return fmt.Errorf("insert wellness entry: %w", err)
	}
	return nil
}

func (r *Repository) ListWellness(ctx context.Context, actorID string, filter ListFilter) (WellnessListResult, error) {
	args := []any{actorID}
	where := `user_id=$1`
	if filter.From != nil {
		args = append(args, filter.From.UTC())
		where += fmt.Sprintf(" AND observed_at >= $%d", len(args))
	}
	if filter.To != nil {
		args = append(args, filter.To.UTC())
		where += fmt.Sprintf(" AND observed_at < $%d", len(args))
	}
	if filter.Cursor != nil {
		at, _ := time.Parse(time.RFC3339Nano, filter.Cursor.At)
		args = append(args, at.UTC(), filter.Cursor.ID)
		where += fmt.Sprintf(" AND (observed_at,id) < ($%d,$%d::uuid)", len(args)-1, len(args))
	}
	args = append(args, filter.Limit+1)
	rows, err := r.pool.Query(ctx, `
		SELECT id,observed_at,day_start_at,timezone_at_entry,sleep_minutes,
		       sleep_quality,energy_level,steps,calories_kcal,protein_g,fat_g,carbs_g,
		       notes,version,created_at,updated_at
		FROM daily_wellness WHERE `+where+
		fmt.Sprintf(" ORDER BY observed_at DESC,id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return WellnessListResult{}, fmt.Errorf("list wellness entries: %w", err)
	}
	defer rows.Close()
	result := WellnessListResult{Items: []WellnessEntry{}}
	for rows.Next() {
		value, err := scanWellness(rows)
		if err != nil {
			return WellnessListResult{}, err
		}
		result.Items = append(result.Items, value)
	}
	if err := rows.Err(); err != nil {
		return WellnessListResult{}, fmt.Errorf("iterate wellness entries: %w", err)
	}
	if len(result.Items) > filter.Limit {
		result.Items = result.Items[:filter.Limit]
		cursor, err := encodeCursor(result.Items[len(result.Items)-1].ObservedAt, result.Items[len(result.Items)-1].ID)
		if err != nil {
			return WellnessListResult{}, err
		}
		result.NextCursor = &cursor
	}
	return result, nil
}

func (r *Repository) WeightSummary(ctx context.Context, actorID string, now time.Time) (WeightSummary, error) {
	var result WeightSummary
	err := r.pool.QueryRow(ctx, `
		WITH current AS (
			SELECT weight_kg::double precision AS value FROM body_measurements
			WHERE user_id=$1 AND measured_at <= $2 AND weight_kg IS NOT NULL
			ORDER BY measured_at DESC,id DESC LIMIT 1
		), baseline7 AS (
			SELECT weight_kg::double precision AS value FROM body_measurements
			WHERE user_id=$1 AND measured_at <= $2 - interval '7 days' AND weight_kg IS NOT NULL
			ORDER BY measured_at DESC,id DESC LIMIT 1
		), baseline30 AS (
			SELECT weight_kg::double precision AS value FROM body_measurements
			WHERE user_id=$1 AND measured_at <= $2 - interval '30 days' AND weight_kg IS NOT NULL
			ORDER BY measured_at DESC,id DESC LIMIT 1
		)
		SELECT current.value,
		       CASE WHEN baseline7.value IS NULL THEN NULL ELSE current.value-baseline7.value END,
		       CASE WHEN baseline30.value IS NULL THEN NULL ELSE current.value-baseline30.value END,
		       (SELECT avg(weight_kg)::double precision FROM body_measurements
		        WHERE user_id=$1 AND measured_at > $2-interval '7 days' AND measured_at <= $2 AND weight_kg IS NOT NULL)
		FROM (SELECT 1) AS seed LEFT JOIN current ON true LEFT JOIN baseline7 ON true LEFT JOIN baseline30 ON true`, actorID, now.UTC()).Scan(
		&result.CurrentKG, &result.Change7DKG, &result.Change30DKG, &result.MovingAverage7D)
	if err != nil {
		return WeightSummary{}, fmt.Errorf("calculate weight summary: %w", err)
	}
	return roundWeightSummary(result), nil
}

func (r *Repository) WeightPoints(ctx context.Context, actorID string, from, to time.Time) ([]WeightPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT sample.measured_at, sample.weight_kg::double precision,
		       (SELECT avg(sample_window.weight_kg)::double precision FROM body_measurements AS sample_window
		        WHERE sample_window.user_id=sample.user_id AND sample_window.weight_kg IS NOT NULL
		          AND sample_window.measured_at > sample.measured_at-interval '7 days'
		          AND sample_window.measured_at <= sample.measured_at)
		FROM body_measurements AS sample
		WHERE sample.user_id=$1 AND sample.measured_at >= $2 AND sample.measured_at < $3
		  AND sample.weight_kg IS NOT NULL
		ORDER BY sample.measured_at,sample.id`, actorID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query weight progress: %w", err)
	}
	defer rows.Close()
	result := []WeightPoint{}
	for rows.Next() {
		var point WeightPoint
		if err := rows.Scan(&point.At, &point.WeightKG, &point.MovingAverage7D); err != nil {
			return nil, fmt.Errorf("scan weight progress: %w", err)
		}
		point.At = point.At.UTC()
		point.WeightKG = round3(point.WeightKG)
		point.MovingAverage7D = round3(point.MovingAverage7D)
		result = append(result, point)
	}
	return result, rows.Err()
}

func (r *Repository) WeightTrend(ctx context.Context, query measurementQuery, actorID string, from, to time.Time) (WeightTrend, error) {
	var result WeightTrend
	err := query.QueryRow(ctx, `
		WITH samples AS (
			SELECT measured_at,id,weight_kg::double precision AS weight FROM body_measurements
			WHERE user_id=$1 AND measured_at >= $2 AND measured_at < $3 AND weight_kg IS NOT NULL
		), edges AS (
			SELECT count(*)::int AS samples,
			       (array_agg(weight ORDER BY measured_at,id))[1] AS start_weight,
			       (array_agg(weight ORDER BY measured_at DESC,id DESC))[1] AS end_weight
			FROM samples
		)
		SELECT samples,start_weight,end_weight,
		       CASE WHEN samples < 2 THEN NULL ELSE end_weight-start_weight END FROM edges`,
		actorID, from.UTC(), to.UTC()).Scan(&result.Samples, &result.StartKG, &result.EndKG, &result.ChangeKG)
	if err != nil {
		return WeightTrend{}, fmt.Errorf("calculate report weight trend: %w", err)
	}
	roundPointers(result.StartKG, result.EndKG, result.ChangeKG)
	return result, nil
}

func (r *Repository) WellnessSummary(ctx context.Context, query measurementQuery, actorID string, from, to time.Time) (WellnessSummary, error) {
	var result WellnessSummary
	var sleepMinutes *float64
	err := query.QueryRow(ctx, `
		SELECT count(*)::int, avg(sleep_minutes)::double precision,
		       avg(sleep_quality)::double precision,avg(energy_level)::double precision,
		       COALESCE(sum(steps),0)::bigint,avg(calories_kcal)::double precision,
		       avg(protein_g)::double precision,avg(fat_g)::double precision,avg(carbs_g)::double precision
		FROM daily_wellness WHERE user_id=$1 AND day_start_at >= $2 AND day_start_at < $3`,
		actorID, from.UTC(), to.UTC()).Scan(&result.Entries, &sleepMinutes,
		&result.AverageSleepQuality, &result.AverageEnergy, &result.TotalSteps,
		&result.AverageCaloriesKcal, &result.AverageProteinG, &result.AverageFatG, &result.AverageCarbsG)
	if err != nil {
		return WellnessSummary{}, fmt.Errorf("calculate report wellness summary: %w", err)
	}
	if sleepMinutes != nil {
		hours := *sleepMinutes / 60
		result.AverageSleepHours = &hours
	}
	roundPointers(result.AverageSleepHours, result.AverageSleepQuality, result.AverageEnergy,
		result.AverageCaloriesKcal, result.AverageProteinG, result.AverageFatG, result.AverageCarbsG)
	return result, nil
}

type scanner interface{ Scan(...any) error }

func scanBody(row scanner) (BodyMeasurement, error) {
	var value BodyMeasurement
	err := row.Scan(&value.ID, &value.MeasuredAt, &value.WeightKG, &value.ChestCM,
		&value.WaistCM, &value.HipsCM, &value.NeckCM, &value.LeftUpperArmCM,
		&value.RightUpperArmCM, &value.LeftThighCM, &value.RightThighCM,
		&value.BodyFatPercent, &value.Notes, &value.Source, &value.Version,
		&value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BodyMeasurement{}, ErrNotFound
	}
	if err != nil {
		return BodyMeasurement{}, fmt.Errorf("scan body measurement: %w", err)
	}
	value.MeasuredAt, value.CreatedAt, value.UpdatedAt = value.MeasuredAt.UTC(), value.CreatedAt.UTC(), value.UpdatedAt.UTC()
	return value, nil
}

func scanWellness(row scanner) (WellnessEntry, error) {
	var value WellnessEntry
	err := row.Scan(&value.ID, &value.ObservedAt, &value.DayStartAt, &value.TimezoneAtEntry,
		&value.SleepMinutes, &value.SleepQuality, &value.Energy, &value.Steps,
		&value.CaloriesKcal, &value.ProteinG, &value.FatG, &value.CarbsG,
		&value.Notes, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return WellnessEntry{}, fmt.Errorf("scan wellness entry: %w", err)
	}
	value.ObservedAt, value.DayStartAt = value.ObservedAt.UTC(), value.DayStartAt.UTC()
	value.CreatedAt, value.UpdatedAt = value.CreatedAt.UTC(), value.UpdatedAt.UTC()
	return value, nil
}

func bodyWriteError(operation string, err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "body_measurements_user_measured_unique" {
		return ErrInstantConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func roundWeightSummary(value WeightSummary) WeightSummary {
	roundPointers(value.CurrentKG, value.Change7DKG, value.Change30DKG, value.MovingAverage7D)
	return value
}

func roundPointers(values ...*float64) {
	for _, value := range values {
		if value != nil {
			*value = round3(*value)
		}
	}
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}
