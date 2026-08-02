package workout

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type PainMessage struct {
	WorkoutID  string    `json:"workout_id"`
	At         time.Time `json:"at"`
	Discomfort *string   `json:"discomfort"`
	Comment    *string   `json:"comment"`
}

type ExerciseSummary struct {
	ExerciseID       string   `json:"exercise_id"`
	ExerciseName     string   `json:"exercise_name"`
	WorkingSets      int      `json:"working_sets"`
	Repetitions      int64    `json:"repetitions"`
	VolumeKG         float64  `json:"volume_kg"`
	MaxWeightKG      *float64 `json:"max_weight_kg"`
	BestEstimated1RM *float64 `json:"best_estimated_1rm_kg"`
}

type AnalyticsSnapshot struct {
	CompletedWorkouts int               `json:"completed_workouts"`
	TrainingDays      int               `json:"training_days"`
	WorkingSets       int               `json:"working_sets"`
	Repetitions       int64             `json:"repetitions"`
	VolumeKG          float64           `json:"volume_kg"`
	ExerciseCount     int               `json:"exercise_count"`
	AverageDifficulty *float64          `json:"average_difficulty"`
	Exercises         []ExerciseSummary `json:"exercise_summaries"`
	PainMessages      []PainMessage     `json:"pain_messages"`
}

type DashboardStats struct {
	Week             AnalyticsSnapshot `json:"week"`
	TotalVolumeKG    float64           `json:"total_volume_kg"`
	NextPlanned      *PlannedWorkout   `json:"next_planned_workout"`
	ActivityWeekKeys []string          `json:"-"`
}

type PlannedWorkout struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ScheduledAt time.Time `json:"scheduled_at"`
}

type ExerciseProgressPoint struct {
	WorkoutID          string    `json:"workout_id"`
	At                 time.Time `json:"at"`
	WorkingSets        int       `json:"working_sets"`
	Repetitions        int64     `json:"repetitions"`
	VolumeKG           float64   `json:"volume_kg"`
	MaxWeightKG        *float64  `json:"max_weight_kg"`
	BestEstimated1RMKG *float64  `json:"estimated_1rm_kg"`
}

type RecordCandidate struct {
	SetID       string
	ExerciseID  string
	WeightKG    *float64
	WeightKey   *string
	Repetitions *int16
	AchievedAt  time.Time
}

func (s *Service) DashboardAnalytics(ctx context.Context, actorID, timezone string, weekStart, weekEnd, now time.Time) (DashboardStats, error) {
	week, err := s.repository.AnalyticsSnapshot(ctx, s.pool, actorID, timezone, weekStart, weekEnd)
	if err != nil {
		return DashboardStats{}, err
	}
	total, err := s.repository.TotalVolume(ctx, actorID)
	if err != nil {
		return DashboardStats{}, err
	}
	next, err := s.repository.NextPlanned(ctx, actorID, now)
	if err != nil {
		return DashboardStats{}, err
	}
	keys, err := s.repository.ActivityWeekKeys(ctx, actorID, timezone, weekStart.AddDate(-10, 0, 0), weekEnd)
	if err != nil {
		return DashboardStats{}, err
	}
	return DashboardStats{Week: week, TotalVolumeKG: roundMetric(total), NextPlanned: next, ActivityWeekKeys: keys}, nil
}

func (s *Service) ExerciseAnalytics(ctx context.Context, actorID, exerciseID string, from, to time.Time) ([]ExerciseProgressPoint, error) {
	return s.repository.ExerciseProgress(ctx, actorID, exerciseID, from, to)
}

func (s *Service) WeeklySnapshot(ctx context.Context, tx pgx.Tx, actorID, timezone string, from, to time.Time) (AnalyticsSnapshot, error) {
	return s.repository.AnalyticsSnapshot(ctx, tx, actorID, timezone, from, to)
}

func (r *Repository) AnalyticsSnapshot(ctx context.Context, query aggregateQuery, actorID, timezone string, from, to time.Time) (AnalyticsSnapshot, error) {
	var result AnalyticsSnapshot
	err := query.QueryRow(ctx, `
		WITH selected_workouts AS (
			SELECT * FROM workouts WHERE user_id=$1 AND status='completed' AND started_at >= $2 AND started_at < $3
		), set_stats AS (
			SELECT count(s.id) FILTER (WHERE s.status='completed' AND s.set_type<>'warmup')::int AS working_sets,
			       COALESCE(sum(s.reps) FILTER (WHERE s.status='completed' AND s.set_type<>'warmup' AND s.reps IS NOT NULL),0)::bigint AS repetitions,
			       COALESCE(sum(s.weight_kg*s.reps) FILTER (WHERE s.status='completed' AND s.set_type<>'warmup' AND s.weight_kg IS NOT NULL AND s.reps IS NOT NULL),0)::double precision AS volume,
			       count(DISTINCT item.exercise_id)::int AS exercise_count
			FROM selected_workouts w LEFT JOIN workout_exercises item ON item.workout_id=w.id AND item.user_id=w.user_id
			LEFT JOIN workout_sets s ON s.workout_exercise_id=item.id AND s.user_id=item.user_id
		)
		SELECT (SELECT count(*)::int FROM selected_workouts),
		       (SELECT count(DISTINCT (started_at AT TIME ZONE $4)::date)::int FROM selected_workouts),
		       set_stats.working_sets,set_stats.repetitions,set_stats.volume,set_stats.exercise_count,
		       (SELECT avg(difficulty)::double precision FROM selected_workouts)
		FROM set_stats`,
		actorID, from.UTC(), to.UTC(), timezone).Scan(&result.CompletedWorkouts, &result.TrainingDays,
		&result.WorkingSets, &result.Repetitions, &result.VolumeKG, &result.ExerciseCount, &result.AverageDifficulty)
	if err != nil {
		return AnalyticsSnapshot{}, fmt.Errorf("calculate workout analytics: %w", err)
	}
	result.VolumeKG = roundMetric(result.VolumeKG)
	if result.AverageDifficulty != nil {
		v := roundMetric(*result.AverageDifficulty)
		result.AverageDifficulty = &v
	}
	rows, err := query.Query(ctx, `
		SELECT item.exercise_id,(array_agg(item.exercise_name_snapshot ORDER BY w.started_at DESC,w.id DESC,item.position))[1],
		       count(s.id)::int,COALESCE(sum(s.reps),0)::bigint,
		       COALESCE(sum(s.weight_kg*s.reps),0)::double precision,max(s.weight_kg)::double precision,
		       max(s.weight_kg*(1+s.reps::numeric/30)) FILTER(WHERE s.weight_kg>0 AND s.reps BETWEEN 1 AND 15)::double precision
		FROM workouts w JOIN workout_exercises item ON item.workout_id=w.id AND item.user_id=w.user_id
		JOIN workout_sets s ON s.workout_exercise_id=item.id AND s.user_id=item.user_id
		WHERE w.user_id=$1 AND w.status='completed' AND w.started_at >= $2 AND w.started_at < $3
		  AND s.status='completed' AND s.set_type<>'warmup'
		GROUP BY item.exercise_id ORDER BY item.exercise_id`, actorID, from.UTC(), to.UTC())
	if err != nil {
		return AnalyticsSnapshot{}, fmt.Errorf("summarize workout exercises: %w", err)
	}
	result.Exercises = []ExerciseSummary{}
	for rows.Next() {
		var item ExerciseSummary
		if err := rows.Scan(&item.ExerciseID, &item.ExerciseName, &item.WorkingSets, &item.Repetitions, &item.VolumeKG, &item.MaxWeightKG, &item.BestEstimated1RM); err != nil {
			rows.Close()
			return AnalyticsSnapshot{}, fmt.Errorf("scan exercise summary: %w", err)
		}
		item.VolumeKG = roundMetric(item.VolumeKG)
		roundFloatPointers(item.MaxWeightKG, item.BestEstimated1RM)
		result.Exercises = append(result.Exercises, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return AnalyticsSnapshot{}, fmt.Errorf("iterate exercise summaries: %w", err)
	}
	rows.Close()
	rows, err = query.Query(ctx, `SELECT id,started_at,discomfort,notes FROM workouts WHERE user_id=$1 AND status='completed' AND started_at >= $2 AND started_at < $3 AND has_pain=true ORDER BY started_at,id`, actorID, from.UTC(), to.UTC())
	if err != nil {
		return AnalyticsSnapshot{}, fmt.Errorf("list workout pain messages: %w", err)
	}
	result.PainMessages = []PainMessage{}
	for rows.Next() {
		var p PainMessage
		if err := rows.Scan(&p.WorkoutID, &p.At, &p.Discomfort, &p.Comment); err != nil {
			rows.Close()
			return AnalyticsSnapshot{}, fmt.Errorf("scan pain message: %w", err)
		}
		p.At = p.At.UTC()
		result.PainMessages = append(result.PainMessages, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return AnalyticsSnapshot{}, err
	}
	rows.Close()
	return result, nil
}

func (r *Repository) TotalVolume(ctx context.Context, actorID string) (float64, error) {
	var value float64
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(sum(s.weight_kg*s.reps) FILTER(WHERE s.status='completed' AND s.set_type<>'warmup' AND s.weight_kg IS NOT NULL AND s.reps IS NOT NULL),0)::double precision FROM workouts w LEFT JOIN workout_exercises item ON item.workout_id=w.id AND item.user_id=w.user_id LEFT JOIN workout_sets s ON s.workout_exercise_id=item.id AND s.user_id=item.user_id WHERE w.user_id=$1 AND w.status='completed'`, actorID).Scan(&value)
	return value, err
}

func (r *Repository) NextPlanned(ctx context.Context, actorID string, now time.Time) (*PlannedWorkout, error) {
	var value PlannedWorkout
	err := r.pool.QueryRow(ctx, `SELECT id,name,scheduled_at FROM workouts WHERE user_id=$1 AND status='planned' AND scheduled_at >= $2 ORDER BY scheduled_at,id LIMIT 1`, actorID, now.UTC()).Scan(&value.ID, &value.Name, &value.ScheduledAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find next planned workout: %w", err)
	}
	value.ScheduledAt = value.ScheduledAt.UTC()
	return &value, nil
}

func (r *Repository) ActivityWeekKeys(ctx context.Context, actorID, timezone string, from, to time.Time) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT to_char(date_trunc('week',started_at AT TIME ZONE $2),'YYYY-MM-DD') FROM workouts WHERE user_id=$1 AND status='completed' AND started_at >= $3 AND started_at < $4 ORDER BY 1 DESC`, actorID, timezone, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query workout activity weeks: %w", err)
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		result = append(result, key)
	}
	return result, rows.Err()
}

func (r *Repository) ExerciseProgress(ctx context.Context, actorID, exerciseID string, from, to time.Time) ([]ExerciseProgressPoint, error) {
	rows, err := r.pool.Query(ctx, `
	SELECT w.id,w.started_at,count(s.id)::int,COALESCE(sum(s.reps),0)::bigint,COALESCE(sum(s.weight_kg*s.reps),0)::double precision,max(s.weight_kg)::double precision,max(s.weight_kg*(1+s.reps::numeric/30)) FILTER(WHERE s.weight_kg>0 AND s.reps BETWEEN 1 AND 15)::double precision
	FROM workouts w JOIN workout_exercises item ON item.workout_id=w.id AND item.user_id=w.user_id JOIN workout_sets s ON s.workout_exercise_id=item.id AND s.user_id=item.user_id
	WHERE w.user_id=$1 AND item.exercise_id=$2 AND w.status='completed' AND w.started_at >= $3 AND w.started_at < $4 AND s.status='completed' AND s.set_type<>'warmup'
	GROUP BY w.id,w.started_at ORDER BY w.started_at,w.id`, actorID, exerciseID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query exercise progress: %w", err)
	}
	defer rows.Close()
	result := []ExerciseProgressPoint{}
	for rows.Next() {
		var p ExerciseProgressPoint
		if err := rows.Scan(&p.WorkoutID, &p.At, &p.WorkingSets, &p.Repetitions, &p.VolumeKG, &p.MaxWeightKG, &p.BestEstimated1RMKG); err != nil {
			return nil, err
		}
		p.At = p.At.UTC()
		p.VolumeKG = roundMetric(p.VolumeKG)
		roundFloatPointers(p.MaxWeightKG, p.BestEstimated1RMKG)
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *Repository) RecordCandidates(ctx context.Context, tx pgx.Tx, actorID string) ([]RecordCandidate, error) {
	rows, err := tx.Query(ctx, `
	SELECT s.id,item.exercise_id,s.weight_kg::double precision,s.weight_kg::text,s.reps,s.completed_at
	FROM workouts w JOIN workout_exercises item ON item.workout_id=w.id AND item.user_id=w.user_id JOIN workout_sets s ON s.workout_exercise_id=item.id AND s.user_id=item.user_id
	WHERE w.user_id=$1 AND w.status='completed' AND s.status='completed' AND s.set_type<>'warmup'
	ORDER BY s.completed_at,w.id,item.position,s.position,s.id`, actorID)
	if err != nil {
		return nil, fmt.Errorf("query personal-record candidates: %w", err)
	}
	defer rows.Close()
	result := []RecordCandidate{}
	for rows.Next() {
		var c RecordCandidate
		if err := rows.Scan(&c.SetID, &c.ExerciseID, &c.WeightKG, &c.WeightKey, &c.Repetitions, &c.AchievedAt); err != nil {
			return nil, err
		}
		c.AchievedAt = c.AchievedAt.UTC()
		result = append(result, c)
	}
	return result, rows.Err()
}

func roundFloatPointers(values ...*float64) {
	for _, value := range values {
		if value != nil {
			*value = roundMetric(*value)
		}
	}
}
