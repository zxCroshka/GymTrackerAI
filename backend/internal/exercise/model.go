package exercise

import (
	"bytes"
	"encoding/json"
	"time"
)

type Exercise struct {
	ID                 string     `json:"id"`
	OwnerUserID        *string    `json:"owner_user_id"`
	Name               string     `json:"name"`
	Description        *string    `json:"description"`
	Instructions       *string    `json:"instructions"`
	PrimaryMuscleGroup *string    `json:"primary_muscle_group"`
	ExerciseType       string     `json:"exercise_type"`
	Equipment          *string    `json:"equipment"`
	MovementPattern    *string    `json:"movement_pattern"`
	IsUnilateral       bool       `json:"is_unilateral"`
	TracksWeight       bool       `json:"tracks_weight"`
	TracksRepetitions  bool       `json:"tracks_repetitions"`
	TracksTime         bool       `json:"tracks_time"`
	TracksDistance     bool       `json:"tracks_distance"`
	Version            int64      `json:"version"`
	ArchivedAt         *time.Time `json:"archived_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CreateInput struct {
	Name               string  `json:"name"`
	Description        *string `json:"description"`
	Instructions       *string `json:"instructions"`
	PrimaryMuscleGroup *string `json:"primary_muscle_group"`
	ExerciseType       string  `json:"exercise_type"`
	Equipment          *string `json:"equipment"`
	MovementPattern    *string `json:"movement_pattern"`
	IsUnilateral       bool    `json:"is_unilateral"`
	TracksWeight       bool    `json:"tracks_weight"`
	TracksRepetitions  bool    `json:"tracks_repetitions"`
	TracksTime         bool    `json:"tracks_time"`
	TracksDistance     bool    `json:"tracks_distance"`
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
	Name               Optional[string] `json:"name"`
	Description        Optional[string] `json:"description"`
	Instructions       Optional[string] `json:"instructions"`
	PrimaryMuscleGroup Optional[string] `json:"primary_muscle_group"`
	ExerciseType       Optional[string] `json:"exercise_type"`
	Equipment          Optional[string] `json:"equipment"`
	MovementPattern    Optional[string] `json:"movement_pattern"`
	IsUnilateral       Optional[bool]   `json:"is_unilateral"`
	TracksWeight       Optional[bool]   `json:"tracks_weight"`
	TracksRepetitions  Optional[bool]   `json:"tracks_repetitions"`
	TracksTime         Optional[bool]   `json:"tracks_time"`
	TracksDistance     Optional[bool]   `json:"tracks_distance"`
}

type ListFilter struct {
	Query, Scope, MuscleGroup, ExerciseType, Equipment string
	IncludeArchived                                    bool
	TracksWeight, TracksRepetitions                    *bool
	TracksTime, TracksDistance                         *bool
	Limit                                              int
	Cursor                                             *Cursor
}

type ListResult struct {
	Items      []Exercise
	NextCursor *string
}

type Cursor struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
