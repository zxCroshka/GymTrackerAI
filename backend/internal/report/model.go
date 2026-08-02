package report

import (
	"errors"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/progress"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

const MetricsSchemaVersion = 1

var (
	ErrValidation = errors.New("report validation failed")
	ErrNotFound   = errors.New("report not found")
)

type WeeklyReport struct {
	ID                   string         `json:"id"`
	PeriodStartAt        time.Time      `json:"period_start_at"`
	PeriodEndAt          time.Time      `json:"period_end_at"`
	Timezone             string         `json:"timezone"`
	Revision             int16          `json:"revision"`
	IsCurrent            bool           `json:"is_current"`
	SupersedesReportID   *string        `json:"supersedes_report_id"`
	Status               string         `json:"status"`
	MetricsSchemaVersion int16          `json:"metrics_schema_version"`
	Metrics              *WeeklyMetrics `json:"metrics"`
	InputDataThroughAt   *time.Time     `json:"input_data_through_at"`
	AIInsightStatus      string         `json:"ai_insight_status"`
	GeneratedAt          *time.Time     `json:"generated_at"`
	Version              int64          `json:"version"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type GenerateInput struct {
	WeekContainingAt *time.Time `json:"week_containing_at"`
}
type Totals struct {
	CompletedWorkouts int     `json:"completed_workouts"`
	WorkingSets       int     `json:"working_sets"`
	Repetitions       int64   `json:"repetitions"`
	VolumeKG          float64 `json:"volume_kg"`
}
type PreviousWeekComparison struct {
	CompletedWorkouts   int      `json:"completed_workouts"`
	VolumeKG            float64  `json:"volume_kg"`
	WorkoutChange       int      `json:"workout_change"`
	VolumeChangeKG      float64  `json:"volume_change_kg"`
	VolumeChangePercent *float64 `json:"volume_change_percent"`
}
type AggregatedIndicators struct {
	TrainingDays        int      `json:"training_days"`
	ExerciseCount       int      `json:"exercise_count"`
	AverageDifficulty   *float64 `json:"average_difficulty"`
	TotalSteps          int64    `json:"total_steps"`
	AverageCaloriesKcal *float64 `json:"average_calories_kcal"`
	AverageProteinG     *float64 `json:"average_protein_g"`
	AverageFatG         *float64 `json:"average_fat_g"`
	AverageCarbsG       *float64 `json:"average_carbs_g"`
}
type WeeklyMetrics struct {
	Totals            Totals                      `json:"totals"`
	PreviousWeek      PreviousWeekComparison      `json:"previous_week"`
	Weight            measurement.WeightTrend     `json:"weight"`
	Wellness          measurement.WellnessSummary `json:"wellness"`
	ExerciseSummaries []workout.ExerciseSummary   `json:"exercise_summaries"`
	NewRecords        []progress.PersonalRecord   `json:"new_records"`
	PainMessages      []workout.PainMessage       `json:"pain_messages"`
	Aggregated        AggregatedIndicators        `json:"aggregated"`
	Coverage          Coverage                    `json:"coverage"`
}
type Coverage struct {
	InputDataThroughAt time.Time `json:"input_data_through_at"`
	HasWeightData      bool      `json:"has_weight_data"`
	HasWellnessData    bool      `json:"has_wellness_data"`
	HasWorkoutData     bool      `json:"has_workout_data"`
}
type ListFilter struct {
	From, To         *time.Time
	Status           string
	IncludeRevisions bool
	Limit            int
}
