package program

import (
	"bytes"
	"encoding/json"
	"time"
)

type Program struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   *string      `json:"description"`
	Goal          *string      `json:"goal"`
	Status        string       `json:"status"`
	Version       int64        `json:"version"`
	ActivatedAt   *time.Time   `json:"activated_at"`
	InactivatedAt *time.Time   `json:"inactivated_at"`
	ArchivedAt    *time.Time   `json:"archived_at"`
	Days          []ProgramDay `json:"days,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type ProgramDay struct {
	ID        string               `json:"id"`
	Position  int16                `json:"position"`
	Name      string               `json:"name"`
	Notes     *string              `json:"notes"`
	Exercises []ProgramDayExercise `json:"exercises"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

type ProgramDayExercise struct {
	ID            string    `json:"id"`
	ExerciseID    string    `json:"exercise_id"`
	Position      int16     `json:"position"`
	WorkingSets   int16     `json:"working_sets"`
	TargetRepsMin *int16    `json:"target_reps_min"`
	TargetRepsMax *int16    `json:"target_reps_max"`
	TargetRIR     *float64  `json:"target_rir"`
	RestSeconds   *int32    `json:"rest_seconds"`
	Notes         *string   `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateInput struct {
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Goal        *string    `json:"goal"`
	Days        []DayInput `json:"days"`
}

type DayInput struct {
	Position  int16              `json:"position"`
	Name      string             `json:"name"`
	Notes     *string            `json:"notes"`
	Exercises []DayExerciseInput `json:"exercises"`
}

type DayExerciseInput struct {
	ExerciseID    string   `json:"exercise_id"`
	Position      int16    `json:"position"`
	WorkingSets   int16    `json:"working_sets"`
	TargetRepsMin *int16   `json:"target_reps_min"`
	TargetRepsMax *int16   `json:"target_reps_max"`
	TargetRIR     *float64 `json:"target_rir"`
	RestSeconds   *int32   `json:"rest_seconds"`
	Notes         *string  `json:"notes"`
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

type PatchInput struct {
	Name        Optional[string]     `json:"name"`
	Description Optional[string]     `json:"description"`
	Goal        Optional[string]     `json:"goal"`
	Days        Optional[[]DayInput] `json:"days"`
}

type ListFilter struct {
	Status          string
	IncludeArchived bool
	Limit           int
	Cursor          *Cursor
}

type ListResult struct {
	Items      []Program
	NextCursor *string
}

type Cursor struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        string    `json:"id"`
}
