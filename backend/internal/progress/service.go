package progress

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/calendartime"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

type MeasurementAnalytics interface {
	WeightSummary(context.Context, string, time.Time) (measurement.WeightSummary, error)
	WeightPoints(context.Context, string, time.Time, time.Time) ([]measurement.WeightPoint, error)
}
type WorkoutAnalytics interface {
	DashboardAnalytics(context.Context, string, string, time.Time, time.Time, time.Time) (workout.DashboardStats, error)
	ExerciseAnalytics(context.Context, string, string, time.Time, time.Time) ([]workout.ExerciseProgressPoint, error)
}
type TimezoneReader interface {
	Timezone(context.Context, string) (string, error)
}

type Service struct {
	repository   *Repository
	measurements MeasurementAnalytics
	workouts     WorkoutAnalytics
	timezones    TimezoneReader
	now          func() time.Time
}

func NewService(repository *Repository, measurements MeasurementAnalytics, workouts WorkoutAnalytics, timezones TimezoneReader) *Service {
	return &Service{repository: repository, measurements: measurements, workouts: workouts, timezones: timezones, now: time.Now}
}

func (s *Service) Dashboard(ctx context.Context, actorID string) (Dashboard, error) {
	now := s.now().UTC()
	timezone, err := s.timezones.Timezone(ctx, actorID)
	if err != nil {
		return Dashboard{}, err
	}
	start, end, err := calendartime.WeekContaining(now, timezone)
	if err != nil {
		return Dashboard{}, ErrValidation
	}
	weight, err := s.measurements.WeightSummary(ctx, actorID, now)
	if err != nil {
		return Dashboard{}, err
	}
	training, err := s.workouts.DashboardAnalytics(ctx, actorID, timezone, start, end, now)
	if err != nil {
		return Dashboard{}, err
	}
	achievements, err := s.repository.Achievements(ctx, s.repository.pool, actorID, start, end)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{AsOf: now, Timezone: timezone, WeekStartAt: start, WeekEndAt: end, Weight: weight, WorkoutsThisWeek: training.Week.CompletedWorkouts, TotalVolumeKG: training.TotalVolumeKG, WeeklyVolumeKG: training.Week.VolumeKG, TrainingStreakWeeks: trainingStreak(training.ActivityWeekKeys, start, timezone), NewAchievements: achievements, NextPlannedWorkout: training.NextPlanned, CalculationVersion: CalculationVersion}, nil
}

func (s *Service) Weight(ctx context.Context, actorID string, from, to time.Time) (WeightProgress, error) {
	if !from.Before(to) || to.Sub(from) > 2*365*24*time.Hour {
		return WeightProgress{}, ErrValidation
	}
	summary, err := s.measurements.WeightSummary(ctx, actorID, to)
	if err != nil {
		return WeightProgress{}, err
	}
	points, err := s.measurements.WeightPoints(ctx, actorID, from, to)
	if err != nil {
		return WeightProgress{}, err
	}
	return WeightProgress{From: from.UTC(), To: to.UTC(), Summary: summary, Points: points, CalculationVersion: "weight_moving_average_7d_v1"}, nil
}

func (s *Service) Exercise(ctx context.Context, actorID, exerciseID string, from, to time.Time) (ExerciseProgress, error) {
	if !id.ValidUUID(exerciseID) || !from.Before(to) || to.Sub(from) > 2*365*24*time.Hour {
		return ExerciseProgress{}, ErrValidation
	}
	points, err := s.workouts.ExerciseAnalytics(ctx, actorID, exerciseID, from, to)
	if err != nil {
		return ExerciseProgress{}, err
	}
	return ExerciseProgress{ExerciseID: exerciseID, From: from.UTC(), To: to.UTC(), Points: points, CalculationVersion: "workout_metrics_v1_epley_15"}, nil
}

func (s *Service) PersonalRecords(ctx context.Context, actorID string, filter RecordFilter) ([]PersonalRecord, error) {
	if filter.Limit < 1 || filter.Limit > 100 || filter.ExerciseID != "" && !id.ValidUUID(filter.ExerciseID) || !validRecordType(filter.RecordType) || filter.From != nil && filter.To != nil && !filter.From.Before(*filter.To) {
		return nil, ErrValidation
	}
	return s.repository.Current(ctx, actorID, filter)
}
func (s *Service) WeeklyAchievements(ctx context.Context, tx pgx.Tx, actorID string, from, to time.Time) ([]PersonalRecord, error) {
	return s.repository.Achievements(ctx, tx, actorID, from, to)
}

func validRecordType(value string) bool {
	switch value {
	case "", "max_weight", "max_reps", "max_set_volume", "estimated_1rm":
		return true
	default:
		return false
	}
}

func trainingStreak(keys []string, currentWeekStart time.Time, timezone string) int {
	set := map[string]struct{}{}
	for _, key := range keys {
		set[key] = struct{}{}
	}
	anchor := currentWeekStart
	key, _ := calendartime.WeekStartKey(anchor, timezone)
	if _, ok := set[key]; !ok {
		anchor = anchor.Add(-time.Nanosecond)
		anchor, _, _ = calendartime.WeekContaining(anchor, timezone)
	}
	streak := 0
	for streak < 520 {
		key, _ = calendartime.WeekStartKey(anchor, timezone)
		if _, ok := set[key]; !ok {
			break
		}
		streak++
		anchor = anchor.Add(-time.Nanosecond)
		anchor, _, _ = calendartime.WeekContaining(anchor, timezone)
	}
	return streak
}
