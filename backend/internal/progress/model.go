package progress

import (
	"errors"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

const CalculationVersion = "personal_records_v1_epley_15"

var ErrValidation = errors.New("progress query validation failed")

type PersonalRecord struct {
	ID                 string    `json:"id"`
	ExerciseID         string    `json:"exercise_id"`
	ExerciseName       string    `json:"exercise_name"`
	WorkoutID          string    `json:"workout_id"`
	WorkoutSetID       string    `json:"workout_set_id"`
	RecordType         string    `json:"record_type"`
	Value              float64   `json:"value"`
	Unit               string    `json:"unit"`
	WeightKG           *float64  `json:"weight_kg"`
	CalculationVersion string    `json:"calculation_version"`
	Formula            *string   `json:"formula"`
	AchievedAt         time.Time `json:"achieved_at"`
}

type RecordFilter struct {
	ExerciseID string
	RecordType string
	From, To   *time.Time
	Limit      int
}

type Dashboard struct {
	AsOf                time.Time                 `json:"as_of"`
	Timezone            string                    `json:"timezone"`
	WeekStartAt         time.Time                 `json:"week_start_at"`
	WeekEndAt           time.Time                 `json:"week_end_at"`
	Weight              measurement.WeightSummary `json:"weight"`
	WorkoutsThisWeek    int                       `json:"workouts_this_week"`
	TotalVolumeKG       float64                   `json:"total_volume_kg"`
	WeeklyVolumeKG      float64                   `json:"weekly_volume_kg"`
	TrainingStreakWeeks int                       `json:"training_streak_weeks"`
	NewAchievements     []PersonalRecord          `json:"new_achievements"`
	NextPlannedWorkout  *workout.PlannedWorkout   `json:"next_planned_workout"`
	CalculationVersion  string                    `json:"calculation_version"`
}

type WeightProgress struct {
	From               time.Time                 `json:"from"`
	To                 time.Time                 `json:"to"`
	Summary            measurement.WeightSummary `json:"summary"`
	Points             []measurement.WeightPoint `json:"points"`
	CalculationVersion string                    `json:"calculation_version"`
}

type ExerciseProgress struct {
	ExerciseID         string                          `json:"exercise_id"`
	From               time.Time                       `json:"from"`
	To                 time.Time                       `json:"to"`
	Points             []workout.ExerciseProgressPoint `json:"points"`
	CalculationVersion string                          `json:"calculation_version"`
}

type recordWrite struct {
	ID, UserID, ExerciseID, SetID, RecordType string
	Value                                     float64
	Formula                                   *string
	AchievedAt, CreatedAt                     time.Time
}
