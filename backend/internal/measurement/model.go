package measurement

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrValidation      = errors.New("measurement validation failed")
	ErrNotFound        = errors.New("measurement not found")
	ErrVersionConflict = errors.New("measurement version conflict")
	ErrInstantConflict = errors.New("measurement instant already exists")
	ErrWellnessExists  = errors.New("wellness entry already exists")
)

type BodyMeasurement struct {
	ID              string    `json:"id"`
	MeasuredAt      time.Time `json:"measured_at"`
	WeightKG        *float64  `json:"weight_kg"`
	ChestCM         *float64  `json:"chest_cm"`
	WaistCM         *float64  `json:"waist_cm"`
	HipsCM          *float64  `json:"hips_cm"`
	NeckCM          *float64  `json:"neck_cm"`
	LeftUpperArmCM  *float64  `json:"left_upper_arm_cm"`
	RightUpperArmCM *float64  `json:"right_upper_arm_cm"`
	LeftThighCM     *float64  `json:"left_thigh_cm"`
	RightThighCM    *float64  `json:"right_thigh_cm"`
	BodyFatPercent  *float64  `json:"body_fat_percent"`
	Notes           *string   `json:"notes"`
	Source          string    `json:"source"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type BodyCreateInput struct {
	MeasuredAt      time.Time `json:"measured_at"`
	WeightKG        *float64  `json:"weight_kg"`
	ChestCM         *float64  `json:"chest_cm"`
	WaistCM         *float64  `json:"waist_cm"`
	HipsCM          *float64  `json:"hips_cm"`
	NeckCM          *float64  `json:"neck_cm"`
	LeftUpperArmCM  *float64  `json:"left_upper_arm_cm"`
	RightUpperArmCM *float64  `json:"right_upper_arm_cm"`
	LeftThighCM     *float64  `json:"left_thigh_cm"`
	RightThighCM    *float64  `json:"right_thigh_cm"`
	BodyFatPercent  *float64  `json:"body_fat_percent"`
	Notes           *string   `json:"notes"`
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

type BodyPatchInput struct {
	MeasuredAt      Optional[time.Time] `json:"measured_at"`
	WeightKG        Optional[float64]   `json:"weight_kg"`
	ChestCM         Optional[float64]   `json:"chest_cm"`
	WaistCM         Optional[float64]   `json:"waist_cm"`
	HipsCM          Optional[float64]   `json:"hips_cm"`
	NeckCM          Optional[float64]   `json:"neck_cm"`
	LeftUpperArmCM  Optional[float64]   `json:"left_upper_arm_cm"`
	RightUpperArmCM Optional[float64]   `json:"right_upper_arm_cm"`
	LeftThighCM     Optional[float64]   `json:"left_thigh_cm"`
	RightThighCM    Optional[float64]   `json:"right_thigh_cm"`
	BodyFatPercent  Optional[float64]   `json:"body_fat_percent"`
	Notes           Optional[string]    `json:"notes"`
}

type WellnessEntry struct {
	ID              string    `json:"id"`
	ObservedAt      time.Time `json:"observed_at"`
	DayStartAt      time.Time `json:"day_start_at"`
	TimezoneAtEntry string    `json:"timezone_at_entry"`
	SleepMinutes    *int16    `json:"sleep_minutes"`
	SleepQuality    *int16    `json:"sleep_quality"`
	Energy          *int16    `json:"energy"`
	Steps           *int32    `json:"steps"`
	CaloriesKcal    *float64  `json:"calories_kcal"`
	ProteinG        *float64  `json:"protein_g"`
	FatG            *float64  `json:"fat_g"`
	CarbsG          *float64  `json:"carbs_g"`
	Notes           *string   `json:"notes"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type WellnessCreateInput struct {
	ObservedAt   time.Time `json:"observed_at"`
	SleepMinutes *int16    `json:"sleep_minutes"`
	SleepQuality *int16    `json:"sleep_quality"`
	Energy       *int16    `json:"energy"`
	Steps        *int32    `json:"steps"`
	CaloriesKcal *float64  `json:"calories_kcal"`
	ProteinG     *float64  `json:"protein_g"`
	FatG         *float64  `json:"fat_g"`
	CarbsG       *float64  `json:"carbs_g"`
	Notes        *string   `json:"notes"`
}

type Cursor struct {
	At string `json:"at"`
	ID string `json:"id"`
}

type ListFilter struct {
	From, To *time.Time
	Limit    int
	Cursor   *Cursor
}

type BodyListResult struct {
	Items      []BodyMeasurement
	NextCursor *string
}

type WellnessListResult struct {
	Items      []WellnessEntry
	NextCursor *string
}

type WeightPoint struct {
	At              time.Time `json:"at"`
	WeightKG        float64   `json:"weight_kg"`
	MovingAverage7D float64   `json:"moving_average_7d_kg"`
}

type WeightSummary struct {
	CurrentKG       *float64 `json:"current_kg"`
	Change7DKG      *float64 `json:"change_7d_kg"`
	Change30DKG     *float64 `json:"change_30d_kg"`
	MovingAverage7D *float64 `json:"moving_average_7d_kg"`
}

type WeightTrend struct {
	Samples  int      `json:"samples"`
	StartKG  *float64 `json:"start_kg"`
	EndKG    *float64 `json:"end_kg"`
	ChangeKG *float64 `json:"change_kg"`
}

type WellnessSummary struct {
	Entries             int      `json:"entries"`
	AverageSleepHours   *float64 `json:"average_sleep_hours"`
	AverageSleepQuality *float64 `json:"average_sleep_quality"`
	AverageEnergy       *float64 `json:"average_energy"`
	TotalSteps          int64    `json:"total_steps"`
	AverageCaloriesKcal *float64 `json:"average_calories_kcal"`
	AverageProteinG     *float64 `json:"average_protein_g"`
	AverageFatG         *float64 `json:"average_fat_g"`
	AverageCarbsG       *float64 `json:"average_carbs_g"`
}
