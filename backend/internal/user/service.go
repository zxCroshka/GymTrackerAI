package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

var ErrValidation = errors.New("profile validation failed")

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
	Name              Optional[string]  `json:"name"`
	Sex               Optional[string]  `json:"sex"`
	BirthDate         Optional[string]  `json:"birth_date"`
	HeightCM          Optional[float64] `json:"height_cm"`
	Goal              Optional[string]  `json:"goal"`
	ExperienceLevel   Optional[string]  `json:"experience_level"`
	TrainingFrequency Optional[int16]   `json:"training_frequency"`
	Timezone          Optional[string]  `json:"timezone"`
	UnitSystem        Optional[string]  `json:"unit_system"`
}

type MeasurementsImport struct {
	ChestCM  *float64 `json:"chest_cm"`
	WaistCM  *float64 `json:"waist_cm"`
	HipsCM   *float64 `json:"hips_cm"`
	NeckCM   *float64 `json:"neck_cm"`
	BicepsCM *float64 `json:"biceps_cm"`
}

type ImportInput struct {
	Name              *string             `json:"name"`
	Sex               *string             `json:"sex"`
	HeightCM          *float64            `json:"height_cm"`
	WeightKG          *float64            `json:"weight_kg"`
	Goal              *string             `json:"goal"`
	TrainingFrequency *int16              `json:"training_frequency"`
	ExperienceLevel   *string             `json:"experience_level"`
	SleepHoursAverage *float64            `json:"sleep_hours_average"`
	Measurements      *MeasurementsImport `json:"measurements"`
	Notes             *[]string           `json:"notes"`
}

type InitialMeasurement struct {
	ID, UserID               string
	MeasuredAt               time.Time
	WeightKG                 *float64
	ChestCM, WaistCM, HipsCM *float64
	NeckCM, BicepsCM         *float64
}

type InitialMeasurementWriter interface {
	InsertInitial(context.Context, pgx.Tx, InitialMeasurement) error
}

type InitialMeasurementWriterFunc func(context.Context, pgx.Tx, InitialMeasurement) error

func (f InitialMeasurementWriterFunc) InsertInitial(ctx context.Context, tx pgx.Tx, value InitialMeasurement) error {
	return f(ctx, tx, value)
}

type ImportResult struct {
	Profile              Profile `json:"profile"`
	InitialMeasurementID *string `json:"initial_measurement_id"`
}

type Service struct {
	pool         *pgxpool.Pool
	repository   *Repository
	measurements InitialMeasurementWriter
	now          func() time.Time
	newID        func() (string, error)
}

func NewService(pool *pgxpool.Pool, repository *Repository, measurements InitialMeasurementWriter) *Service {
	return &Service{pool: pool, repository: repository, measurements: measurements, now: time.Now, newID: id.UUID}
}

func (s *Service) Get(ctx context.Context, userID string) (Profile, error) {
	value, err := s.repository.Get(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	return profileFromDatabase(value), nil
}

func (s *Service) Patch(ctx context.Context, userID string, expectedVersion int64, input PatchInput) (Profile, error) {
	if !patchHasFields(input) {
		return Profile{}, ErrValidation
	}
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		profile, err := s.repository.Lock(ctx, tx, userID)
		if err != nil {
			return err
		}
		if profile.Version != expectedVersion {
			return ErrVersionConflict
		}
		if input.Name.Set {
			profile.Name = cleanString(input.Name.Value)
		}
		if input.Sex.Set {
			profile.Sex = cleanString(input.Sex.Value)
		}
		if input.BirthDate.Set {
			profile.BirthDate, err = parseBirthDate(input.BirthDate.Value, now)
			if err != nil {
				return ErrValidation
			}
		}
		if input.HeightCM.Set {
			profile.HeightCM = input.HeightCM.Value
		}
		if input.Goal.Set {
			profile.Goal = cleanString(input.Goal.Value)
		}
		if input.ExperienceLevel.Set {
			profile.ExperienceLevel = cleanString(input.ExperienceLevel.Value)
		}
		if input.TrainingFrequency.Set {
			profile.TrainingFrequency = input.TrainingFrequency.Value
		}
		if input.Timezone.Set {
			if input.Timezone.Value == nil {
				return ErrValidation
			}
			profile.Timezone = strings.TrimSpace(*input.Timezone.Value)
		}
		if input.UnitSystem.Set {
			if input.UnitSystem.Value == nil {
				return ErrValidation
			}
			profile.UnitSystem = strings.TrimSpace(*input.UnitSystem.Value)
		}
		if err := validateProfile(profile, now); err != nil {
			return err
		}
		_, err = s.repository.Update(ctx, tx, profile, expectedVersion, now)
		return err
	})
	if err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID)
}

func (s *Service) Import(ctx context.Context, userID string, expectedVersion int64, input ImportInput) (ImportResult, error) {
	if !importHasFields(input) || validateImport(input) != nil {
		return ImportResult{}, ErrValidation
	}
	now := s.now().UTC()
	var measurementID *string
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		profile, err := s.repository.Lock(ctx, tx, userID)
		if err != nil {
			return err
		}
		if profile.Version != expectedVersion {
			return ErrVersionConflict
		}
		if input.Name != nil {
			profile.Name = cleanString(input.Name)
		}
		if input.Sex != nil {
			profile.Sex = cleanString(input.Sex)
		}
		if input.HeightCM != nil {
			profile.HeightCM = input.HeightCM
		}
		if input.Goal != nil {
			profile.Goal = cleanString(input.Goal)
		}
		if input.TrainingFrequency != nil {
			profile.TrainingFrequency = input.TrainingFrequency
		}
		if input.ExperienceLevel != nil {
			profile.ExperienceLevel = cleanString(input.ExperienceLevel)
		}
		if input.SleepHoursAverage != nil {
			profile.SleepHoursAverage = input.SleepHoursAverage
		}
		if err := validateProfile(profile, now); err != nil {
			return err
		}
		if _, err := s.repository.Update(ctx, tx, profile, expectedVersion, now); err != nil {
			return err
		}
		if input.Notes != nil {
			notes := make([]string, len(*input.Notes))
			for index, note := range *input.Notes {
				notes[index] = strings.TrimSpace(note)
			}
			if err := s.repository.ReplaceNotes(ctx, tx, userID, notes, now); err != nil {
				return err
			}
		}
		if measurementPresent(input) {
			generated, err := s.newID()
			if err != nil {
				return err
			}
			value := InitialMeasurement{ID: generated, UserID: userID, MeasuredAt: now, WeightKG: input.WeightKG}
			if input.Measurements != nil {
				value.ChestCM, value.WaistCM, value.HipsCM = input.Measurements.ChestCM, input.Measurements.WaistCM, input.Measurements.HipsCM
				value.NeckCM, value.BicepsCM = input.Measurements.NeckCM, input.Measurements.BicepsCM
			}
			if err := s.measurements.InsertInitial(ctx, tx, value); err != nil {
				return err
			}
			measurementID = &generated
		}
		return nil
	})
	if err != nil {
		return ImportResult{}, err
	}
	profile, err := s.Get(ctx, userID)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Profile: profile, InitialMeasurementID: measurementID}, nil
}

func patchHasFields(input PatchInput) bool {
	return input.Name.Set || input.Sex.Set || input.BirthDate.Set || input.HeightCM.Set || input.Goal.Set ||
		input.ExperienceLevel.Set || input.TrainingFrequency.Set || input.Timezone.Set || input.UnitSystem.Set
}

func importHasFields(input ImportInput) bool {
	return input.Name != nil || input.Sex != nil || input.HeightCM != nil || input.WeightKG != nil || input.Goal != nil ||
		input.TrainingFrequency != nil || input.ExperienceLevel != nil || input.SleepHoursAverage != nil ||
		input.Measurements != nil || input.Notes != nil
}

func cleanString(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}

func parseBirthDate(value *string, now time.Time) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, *value)
	if err != nil || parsed.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) || parsed.After(now) {
		return nil, ErrValidation
	}
	return &parsed, nil
}

func validateProfile(value databaseProfile, now time.Time) error {
	if value.Name != nil && (len(*value.Name) < 1 || len([]rune(*value.Name)) > 100 || *value.Name != strings.TrimSpace(*value.Name)) {
		return ErrValidation
	}
	if value.Sex != nil && !oneOf(*value.Sex, "male", "female", "other", "prefer_not_to_say") {
		return ErrValidation
	}
	if value.BirthDate != nil && (value.BirthDate.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) || value.BirthDate.After(now)) {
		return ErrValidation
	}
	if !validNumber(value.HeightCM, 50, 300) || !validNumber(value.SleepHoursAverage, 0, 24) {
		return ErrValidation
	}
	if value.Goal != nil && !oneOf(*value.Goal, "muscle_gain", "weight_loss", "recomposition", "strength", "maintenance") {
		return ErrValidation
	}
	if value.ExperienceLevel != nil && !oneOf(*value.ExperienceLevel, "beginner", "intermediate", "advanced") {
		return ErrValidation
	}
	if value.TrainingFrequency != nil && (*value.TrainingFrequency < 1 || *value.TrainingFrequency > 7) {
		return ErrValidation
	}
	if len(value.Timezone) < 1 || len(value.Timezone) > 255 {
		return ErrValidation
	}
	if _, err := time.LoadLocation(value.Timezone); err != nil {
		return ErrValidation
	}
	if !oneOf(value.UnitSystem, "metric", "imperial") {
		return ErrValidation
	}
	return nil
}

func validateImport(input ImportInput) error {
	if input.Notes != nil {
		if len(*input.Notes) > 20 {
			return ErrValidation
		}
		for _, note := range *input.Notes {
			cleaned := strings.TrimSpace(note)
			if len(cleaned) < 1 || len([]rune(cleaned)) > 1000 {
				return ErrValidation
			}
		}
	}
	if !validNumber(input.WeightKG, 20, 700) {
		return ErrValidation
	}
	if input.Measurements != nil {
		if !measurementFieldsPresent(*input.Measurements) {
			return ErrValidation
		}
		for _, check := range []struct {
			value    *float64
			min, max float64
		}{
			{input.Measurements.ChestCM, 5, 400}, {input.Measurements.WaistCM, 5, 400},
			{input.Measurements.HipsCM, 5, 400}, {input.Measurements.NeckCM, 5, 200},
			{input.Measurements.BicepsCM, 5, 200},
		} {
			if !validNumber(check.value, check.min, check.max) {
				return ErrValidation
			}
		}
	}
	return nil
}

func measurementFieldsPresent(value MeasurementsImport) bool {
	return value.ChestCM != nil || value.WaistCM != nil || value.HipsCM != nil ||
		value.NeckCM != nil || value.BicepsCM != nil
}

func measurementPresent(input ImportInput) bool {
	if input.WeightKG != nil {
		return true
	}
	if input.Measurements == nil {
		return false
	}
	return input.Measurements.ChestCM != nil || input.Measurements.WaistCM != nil || input.Measurements.HipsCM != nil ||
		input.Measurements.NeckCM != nil || input.Measurements.BicepsCM != nil
}

func validNumber(value *float64, minimum, maximum float64) bool {
	return value == nil || (!math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= minimum && *value <= maximum)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
