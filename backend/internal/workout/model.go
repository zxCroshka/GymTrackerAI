package workout

import (
	"bytes"
	"encoding/json"
	"time"
)

const CalculationVersion = "workout_metrics_v1"

type Workout struct {
	ID                   string            `json:"id"`
	SourceProgramID      *string           `json:"source_program_id"`
	SourceProgramDayID   *string           `json:"source_program_day_id"`
	SourceProgramVersion *int64            `json:"source_program_version"`
	Name                 string            `json:"name"`
	Status               string            `json:"status"`
	EventAt              time.Time         `json:"event_at"`
	ScheduledAt          *time.Time        `json:"scheduled_at"`
	StartedAt            *time.Time        `json:"started_at"`
	CompletedAt          *time.Time        `json:"completed_at"`
	CancelledAt          *time.Time        `json:"cancelled_at"`
	Difficulty           *int16            `json:"difficulty"`
	Energy               *int16            `json:"energy"`
	Mood                 *int16            `json:"mood"`
	Comment              *string           `json:"comment"`
	HasPain              *bool             `json:"has_pain"`
	Discomfort           *string           `json:"discomfort"`
	ExerciseCount        int               `json:"exercise_count"`
	SetCount             int               `json:"set_count"`
	WorkingSetCount      int               `json:"working_set_count"`
	VolumeKG             float64           `json:"volume_kg"`
	BestEstimated1RMKG   *float64          `json:"best_estimated_1rm_kg"`
	CalculationVersion   string            `json:"calculation_version"`
	Version              int64             `json:"version"`
	Exercises            []WorkoutExercise `json:"exercises,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

type WorkoutExercise struct {
	ID                         string       `json:"id"`
	WorkoutID                  string       `json:"workout_id"`
	ExerciseID                 string       `json:"exercise_id"`
	SourceProgramDayExerciseID *string      `json:"source_program_day_exercise_id"`
	Position                   int16        `json:"position"`
	ExerciseNameSnapshot       string       `json:"exercise_name_snapshot"`
	Comment                    *string      `json:"comment"`
	RestSeconds                *int32       `json:"rest_seconds"`
	TracksWeight               bool         `json:"tracks_weight"`
	TracksRepetitions          bool         `json:"tracks_repetitions"`
	TracksTime                 bool         `json:"tracks_time"`
	TracksDistance             bool         `json:"tracks_distance"`
	VolumeKG                   float64      `json:"volume_kg"`
	BestEstimated1RMKG         *float64     `json:"best_estimated_1rm_kg"`
	Sets                       []WorkoutSet `json:"sets"`
	CreatedAt                  time.Time    `json:"created_at"`
	UpdatedAt                  time.Time    `json:"updated_at"`
}

type WorkoutSet struct {
	ID                string     `json:"id"`
	WorkoutExerciseID string     `json:"workout_exercise_id"`
	SetNumber         int16      `json:"set_number"`
	Status            string     `json:"status"`
	TargetWeightKG    *float64   `json:"target_weight_kg"`
	TargetRepsMin     *int16     `json:"target_reps_min"`
	TargetRepsMax     *int16     `json:"target_reps_max"`
	TargetRIR         *float64   `json:"target_rir"`
	WeightKG          *float64   `json:"weight_kg"`
	Repetitions       *int16     `json:"reps"`
	RIR               *float64   `json:"rir"`
	SetType           string     `json:"-"`
	Warmup            bool       `json:"warmup"`
	Failure           bool       `json:"failure"`
	DurationSeconds   *int32     `json:"duration_seconds"`
	DistanceMeters    *float64   `json:"distance_meters"`
	Note              *string    `json:"note"`
	PerformedAt       *time.Time `json:"performed_at"`
	VolumeKG          *float64   `json:"volume_kg"`
	Estimated1RMKG    *float64   `json:"estimated_1rm_kg"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Optional[T any] struct {
	Set   bool
	Value *T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

type CreateInput struct {
	ProgramDayID *string    `json:"program_day_id"`
	Name         *string    `json:"name"`
	Status       string     `json:"status"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	StartedAt    *time.Time `json:"started_at"`
	Difficulty   *int16     `json:"difficulty"`
	Energy       *int16     `json:"energy"`
	Mood         *int16     `json:"mood"`
	Comment      *string    `json:"comment"`
	HasPain      *bool      `json:"has_pain"`
	Discomfort   *string    `json:"discomfort"`
}

type PatchInput struct {
	Name        Optional[string]    `json:"name"`
	Status      Optional[string]    `json:"status"`
	ScheduledAt Optional[time.Time] `json:"scheduled_at"`
	StartedAt   Optional[time.Time] `json:"started_at"`
	CompletedAt Optional[time.Time] `json:"completed_at"`
	Difficulty  Optional[int16]     `json:"difficulty"`
	Energy      Optional[int16]     `json:"energy"`
	Mood        Optional[int16]     `json:"mood"`
	Comment     Optional[string]    `json:"comment"`
	HasPain     Optional[bool]      `json:"has_pain"`
	Discomfort  Optional[string]    `json:"discomfort"`
}

type CompleteInput struct {
	CompletedAt Optional[time.Time] `json:"completed_at"`
	Difficulty  Optional[int16]     `json:"difficulty"`
	Energy      Optional[int16]     `json:"energy"`
	Mood        Optional[int16]     `json:"mood"`
	Comment     Optional[string]    `json:"comment"`
	HasPain     Optional[bool]      `json:"has_pain"`
	Discomfort  Optional[string]    `json:"discomfort"`
}

type ExerciseCreateInput struct {
	ExerciseID string  `json:"exercise_id"`
	Position   *int16  `json:"position"`
	Comment    *string `json:"comment"`
}

type ExercisePatchInput struct {
	Position Optional[int16]  `json:"position"`
	Comment  Optional[string] `json:"comment"`
}

type SetCreateInput struct {
	SetNumber       *int16     `json:"set_number"`
	WeightKG        *float64   `json:"weight_kg"`
	Repetitions     *int16     `json:"reps"`
	RIR             *float64   `json:"rir"`
	Warmup          bool       `json:"warmup"`
	Failure         bool       `json:"failure"`
	DurationSeconds *int32     `json:"duration_seconds"`
	DistanceMeters  *float64   `json:"distance_meters"`
	Note            *string    `json:"note"`
	PerformedAt     *time.Time `json:"performed_at"`
}

type SetPatchInput struct {
	SetNumber       Optional[int16]     `json:"set_number"`
	WeightKG        Optional[float64]   `json:"weight_kg"`
	Repetitions     Optional[int16]     `json:"reps"`
	RIR             Optional[float64]   `json:"rir"`
	Warmup          Optional[bool]      `json:"warmup"`
	Failure         Optional[bool]      `json:"failure"`
	DurationSeconds Optional[int32]     `json:"duration_seconds"`
	DistanceMeters  Optional[float64]   `json:"distance_meters"`
	Note            Optional[string]    `json:"note"`
	PerformedAt     Optional[time.Time] `json:"performed_at"`
}

type ListFilter struct {
	Status     string
	From       *time.Time
	To         *time.Time
	ProgramID  string
	ExerciseID string
	Limit      int
	Cursor     *Cursor
}

type ListResult struct {
	Items      []Workout
	NextCursor *string
}

type Cursor struct {
	EventAt   time.Time `json:"event_at"`
	ID        string    `json:"id"`
	FilterKey string    `json:"filter_key"`
}

type PreviousResult struct {
	ExerciseID              string       `json:"exercise_id"`
	ExerciseName            string       `json:"exercise_name"`
	SourceWorkoutID         string       `json:"source_workout_id"`
	SourceWorkoutExerciseID string       `json:"source_workout_exercise_id"`
	WorkoutName             string       `json:"workout_name"`
	StartedAt               time.Time    `json:"started_at"`
	CompletedAt             time.Time    `json:"completed_at"`
	Sets                    []WorkoutSet `json:"sets"`
}
