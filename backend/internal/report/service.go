package report

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/calendartime"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/progress"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

type TimezoneReader interface {
	Timezone(context.Context, string) (string, error)
}
type WorkoutSnapshotReader interface {
	WeeklySnapshot(context.Context, pgx.Tx, string, string, time.Time, time.Time) (workout.AnalyticsSnapshot, error)
}
type MeasurementSnapshotReader interface {
	ReportSnapshot(context.Context, pgx.Tx, string, time.Time, time.Time) (measurement.WeightTrend, measurement.WellnessSummary, error)
}
type AchievementReader interface {
	WeeklyAchievements(context.Context, pgx.Tx, string, time.Time, time.Time) ([]progress.PersonalRecord, error)
}

type Service struct {
	pool         *pgxpool.Pool
	repository   *Repository
	source       *SourceInvalidator
	timezones    TimezoneReader
	workouts     WorkoutSnapshotReader
	measurements MeasurementSnapshotReader
	achievements AchievementReader
	now          func() time.Time
	newID        func() (string, error)
}

func NewService(pool *pgxpool.Pool, repository *Repository, source *SourceInvalidator, timezones TimezoneReader, workouts WorkoutSnapshotReader, measurements MeasurementSnapshotReader, achievements AchievementReader) *Service {
	return &Service{pool: pool, repository: repository, source: source, timezones: timezones, workouts: workouts, measurements: measurements, achievements: achievements, now: time.Now, newID: id.UUID}
}

func (s *Service) GenerateWeekly(ctx context.Context, actorID string, input GenerateInput) (WeeklyReport, bool, error) {
	instant := s.now().UTC()
	if input.WeekContainingAt != nil {
		if input.WeekContainingAt.Location() != time.UTC {
			return WeeklyReport{}, false, ErrValidation
		}
		instant = input.WeekContainingAt.UTC()
	}
	timezone, err := s.timezones.Timezone(ctx, actorID)
	if err != nil {
		return WeeklyReport{}, false, err
	}
	start, end, err := calendartime.WeekContaining(instant, timezone)
	if err != nil {
		return WeeklyReport{}, false, ErrValidation
	}
	preflightCutoff := s.now().UTC()
	if preflightCutoff.Before(start) {
		return WeeklyReport{}, false, ErrValidation
	}
	var result WeeklyReport
	created := false
	// READ COMMITTED is intentional: the first statement may wait for a source
	// writer holding the same user row. Its following statements must see that
	// writer's committed facts. While this transaction owns the row lock, all
	// supported source writers are fenced, so the multi-query aggregate remains
	// stable without taking a snapshot before the wait.
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		if err := s.source.LockUser(ctx, tx, actorID); err != nil {
			return err
		}
		current, err := s.repository.CurrentForPeriod(ctx, tx, actorID, start, end)
		if err != nil {
			return err
		}
		if current != nil && current.Status == "ready" {
			result = *current
			return nil
		}
		cutoff := s.now().UTC()
		if cutoff.Before(start) {
			return ErrValidation
		}
		sourceEnd := end
		if cutoff.Before(sourceEnd) {
			sourceEnd = cutoff
		}
		currentWeek, err := s.workouts.WeeklySnapshot(ctx, tx, actorID, timezone, start, sourceEnd)
		if err != nil {
			return err
		}
		previousStart, _, err := calendartime.WeekContaining(start.Add(-time.Nanosecond), timezone)
		if err != nil {
			return err
		}
		previous, err := s.workouts.WeeklySnapshot(ctx, tx, actorID, timezone, previousStart, start)
		if err != nil {
			return err
		}
		weight, wellness, err := s.measurements.ReportSnapshot(ctx, tx, actorID, start, sourceEnd)
		if err != nil {
			return err
		}
		records, err := s.achievements.WeeklyAchievements(ctx, tx, actorID, start, sourceEnd)
		if err != nil {
			return err
		}
		metrics := buildMetrics(currentWeek, previous, weight, wellness, records, cutoff)
		reportID, err := s.newID()
		if err != nil {
			return err
		}
		revision := int16(1)
		var supersedes *string
		if current != nil {
			revision = current.Revision + 1
			supersedes = &current.ID
		}
		now := cutoff
		result = WeeklyReport{ID: reportID, PeriodStartAt: start, PeriodEndAt: end, Timezone: timezone, Revision: revision, IsCurrent: true, SupersedesReportID: supersedes, Status: "ready", MetricsSchemaVersion: MetricsSchemaVersion, Metrics: &metrics, InputDataThroughAt: &cutoff, AIInsightStatus: "not_requested", GeneratedAt: &now, Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := s.repository.InsertReady(ctx, tx, actorID, result); err != nil {
			return err
		}
		created = true
		return nil
	})
	return result, created, err
}

func (s *Service) List(ctx context.Context, actorID string, filter ListFilter) ([]WeeklyReport, error) {
	if filter.Limit < 1 || filter.Limit > 100 || filter.From != nil && filter.To != nil && !filter.From.Before(*filter.To) || !validStatus(filter.Status) {
		return nil, ErrValidation
	}
	return s.repository.List(ctx, actorID, filter)
}
func (s *Service) Get(ctx context.Context, actorID, reportID string) (WeeklyReport, error) {
	if !id.ValidUUID(reportID) {
		return WeeklyReport{}, ErrValidation
	}
	return s.repository.Get(ctx, actorID, reportID)
}

func buildMetrics(current, previous workout.AnalyticsSnapshot, weight measurement.WeightTrend, wellness measurement.WellnessSummary, records []progress.PersonalRecord, cutoff time.Time) WeeklyMetrics {
	change := current.VolumeKG - previous.VolumeKG
	var percent *float64
	if previous.VolumeKG != 0 {
		value := math.Round(change/previous.VolumeKG*100000) / 1000
		percent = &value
	}
	return WeeklyMetrics{Totals: Totals{CompletedWorkouts: current.CompletedWorkouts, WorkingSets: current.WorkingSets, Repetitions: current.Repetitions, VolumeKG: current.VolumeKG}, PreviousWeek: PreviousWeekComparison{CompletedWorkouts: previous.CompletedWorkouts, VolumeKG: previous.VolumeKG, WorkoutChange: current.CompletedWorkouts - previous.CompletedWorkouts, VolumeChangeKG: math.Round(change*1000) / 1000, VolumeChangePercent: percent}, Weight: weight, Wellness: wellness, ExerciseSummaries: nonNilExercises(current.Exercises), NewRecords: nonNilRecords(records), PainMessages: nonNilPain(current.PainMessages), Aggregated: AggregatedIndicators{TrainingDays: current.TrainingDays, ExerciseCount: current.ExerciseCount, AverageDifficulty: current.AverageDifficulty, TotalSteps: wellness.TotalSteps, AverageCaloriesKcal: wellness.AverageCaloriesKcal, AverageProteinG: wellness.AverageProteinG, AverageFatG: wellness.AverageFatG, AverageCarbsG: wellness.AverageCarbsG}, Coverage: Coverage{InputDataThroughAt: cutoff.UTC(), HasWeightData: weight.Samples > 0, HasWellnessData: wellness.Entries > 0, HasWorkoutData: current.CompletedWorkouts > 0}}
}
func nonNilExercises(value []workout.ExerciseSummary) []workout.ExerciseSummary {
	if value == nil {
		return []workout.ExerciseSummary{}
	}
	return value
}
func nonNilRecords(value []progress.PersonalRecord) []progress.PersonalRecord {
	if value == nil {
		return []progress.PersonalRecord{}
	}
	return value
}
func nonNilPain(value []workout.PainMessage) []workout.PainMessage {
	if value == nil {
		return []workout.PainMessage{}
	}
	return value
}
func validStatus(value string) bool {
	switch value {
	case "", "ready", "stale":
		return true
	default:
		return false
	}
}
