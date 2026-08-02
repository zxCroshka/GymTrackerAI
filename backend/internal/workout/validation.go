package workout

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrValidation          = errors.New("workout validation failed")
	ErrNotFound            = errors.New("workout not found")
	ErrExerciseNotFound    = errors.New("workout exercise not found")
	ErrSetNotFound         = errors.New("workout set not found")
	ErrVersionConflict     = errors.New("workout version conflict")
	ErrActiveExists        = errors.New("active workout already exists")
	ErrInvalidState        = errors.New("invalid workout state")
	ErrProgramUnavailable  = errors.New("active program day unavailable")
	ErrExerciseUnavailable = errors.New("exercise unavailable")
	ErrMetricNotTracked    = errors.New("metric is not tracked by exercise")
	ErrExportTooLarge      = errors.New("workout export is too large")
)

func normalizeCreate(input CreateInput, now time.Time) (CreateInput, error) {
	if input.ProgramDayID != nil && input.Name != nil || input.ProgramDayID == nil && input.Name == nil {
		return CreateInput{}, ErrValidation
	}
	if input.ProgramDayID != nil && strings.TrimSpace(*input.ProgramDayID) == "" {
		return CreateInput{}, ErrValidation
	}
	if input.Name != nil {
		name, err := requiredText(*input.Name, 200)
		if err != nil {
			return CreateInput{}, err
		}
		input.Name = &name
	}
	if input.Status == "" {
		input.Status = "in_progress"
	}
	if input.Status != "planned" && input.Status != "in_progress" {
		return CreateInput{}, ErrValidation
	}
	if input.Status == "planned" {
		if input.StartedAt != nil {
			return CreateInput{}, ErrValidation
		}
	} else if input.StartedAt == nil {
		started := now.UTC()
		input.StartedAt = &started
	}
	input.ScheduledAt = utcTime(input.ScheduledAt)
	input.StartedAt = utcTime(input.StartedAt)
	if err := validateScores(input.Difficulty, input.Energy, input.Mood); err != nil {
		return CreateInput{}, err
	}
	var err error
	if input.Comment, err = optionalText(input.Comment, 4000); err != nil {
		return CreateInput{}, err
	}
	if input.Discomfort, err = optionalText(input.Discomfort, 4000); err != nil {
		return CreateInput{}, err
	}
	return input, nil
}

func validatePatch(input PatchInput) error {
	if !workoutPatchHasFields(input) {
		return ErrValidation
	}
	if input.Name.Set {
		if input.Name.Value == nil {
			return ErrValidation
		}
		value, err := requiredText(*input.Name.Value, 200)
		if err != nil {
			return err
		}
		input.Name.Value = &value
	}
	if input.Status.Set && (input.Status.Value == nil || *input.Status.Value != "in_progress" && *input.Status.Value != "cancelled") {
		return ErrValidation
	}
	if input.StartedAt.Set && input.StartedAt.Value == nil || input.CompletedAt.Set && input.CompletedAt.Value == nil {
		return ErrValidation
	}
	if err := validateOptionalScore(input.Difficulty); err != nil {
		return err
	}
	if err := validateOptionalScore(input.Energy); err != nil {
		return err
	}
	if err := validateOptionalScore(input.Mood); err != nil {
		return err
	}
	if err := validateOptionalText(input.Comment, 4000); err != nil {
		return err
	}
	return validateOptionalText(input.Discomfort, 4000)
}

func validateComplete(input CompleteInput) error {
	if input.CompletedAt.Set && input.CompletedAt.Value == nil {
		return ErrValidation
	}
	if err := validateOptionalScore(input.Difficulty); err != nil {
		return err
	}
	if err := validateOptionalScore(input.Energy); err != nil {
		return err
	}
	if err := validateOptionalScore(input.Mood); err != nil {
		return err
	}
	if err := validateOptionalText(input.Comment, 4000); err != nil {
		return err
	}
	return validateOptionalText(input.Discomfort, 4000)
}

func validateExerciseCreate(input ExerciseCreateInput) error {
	if strings.TrimSpace(input.ExerciseID) == "" {
		return ErrValidation
	}
	if input.Position != nil && (*input.Position < 1 || *input.Position > 50) {
		return ErrValidation
	}
	_, err := optionalText(input.Comment, 4000)
	return err
}

func validateExercisePatch(input ExercisePatchInput) error {
	if !input.Position.Set && !input.Comment.Set {
		return ErrValidation
	}
	if input.Position.Set && (input.Position.Value == nil || *input.Position.Value < 1 || *input.Position.Value > 50) {
		return ErrValidation
	}
	return validateOptionalText(input.Comment, 4000)
}

func normalizeSetCreate(input SetCreateInput, capabilities WorkoutExercise, now time.Time) (SetCreateInput, error) {
	if input.SetNumber != nil && (*input.SetNumber < 1 || *input.SetNumber > 100) {
		return SetCreateInput{}, ErrValidation
	}
	if err := validateActualValues(input.WeightKG, input.Repetitions, input.RIR, input.DurationSeconds, input.DistanceMeters); err != nil {
		return SetCreateInput{}, err
	}
	if input.WeightKG == nil && input.Repetitions == nil && input.DurationSeconds == nil && input.DistanceMeters == nil {
		return SetCreateInput{}, ErrValidation
	}
	if input.Warmup && input.Failure {
		return SetCreateInput{}, ErrValidation
	}
	if err := validateCapabilities(capabilities, input.WeightKG, input.Repetitions, input.DurationSeconds, input.DistanceMeters); err != nil {
		return SetCreateInput{}, err
	}
	var err error
	if input.Note, err = optionalText(input.Note, 4000); err != nil {
		return SetCreateInput{}, err
	}
	if input.PerformedAt == nil {
		performed := now.UTC()
		input.PerformedAt = &performed
	} else {
		input.PerformedAt = utcTime(input.PerformedAt)
	}
	return input, nil
}

func validateSetPatch(input SetPatchInput) error {
	if !setPatchHasFields(input) {
		return ErrValidation
	}
	if input.SetNumber.Set && (input.SetNumber.Value == nil || *input.SetNumber.Value < 1 || *input.SetNumber.Value > 100) {
		return ErrValidation
	}
	if input.PerformedAt.Set && input.PerformedAt.Value == nil {
		return ErrValidation
	}
	if input.WeightKG.Set && input.WeightKG.Value != nil && (!finite(*input.WeightKG.Value) || *input.WeightKG.Value < 0 || *input.WeightKG.Value > 99999.999) {
		return ErrValidation
	}
	if input.Repetitions.Set && input.Repetitions.Value != nil && *input.Repetitions.Value < 0 {
		return ErrValidation
	}
	if input.RIR.Set && input.RIR.Value != nil && !validRIR(*input.RIR.Value) {
		return ErrValidation
	}
	if input.DurationSeconds.Set && input.DurationSeconds.Value != nil && (*input.DurationSeconds.Value < 0 || *input.DurationSeconds.Value > 86400) {
		return ErrValidation
	}
	if input.DistanceMeters.Set && input.DistanceMeters.Value != nil && (!finite(*input.DistanceMeters.Value) || *input.DistanceMeters.Value < 0 || *input.DistanceMeters.Value > 999999999.999) {
		return ErrValidation
	}
	return validateOptionalText(input.Note, 4000)
}

func validateActualValues(weight *float64, repetitions *int16, rir *float64, duration *int32, distance *float64) error {
	if weight != nil && (!finite(*weight) || *weight < 0 || *weight > 99999.999) {
		return ErrValidation
	}
	if repetitions != nil && *repetitions < 0 {
		return ErrValidation
	}
	if rir != nil && !validRIR(*rir) {
		return ErrValidation
	}
	if duration != nil && (*duration < 0 || *duration > 86400) {
		return ErrValidation
	}
	if distance != nil && (!finite(*distance) || *distance < 0 || *distance > 999999999.999) {
		return ErrValidation
	}
	return nil
}

func validateCapabilities(capabilities WorkoutExercise, weight *float64, repetitions *int16, duration *int32, distance *float64) error {
	if weight != nil && !capabilities.TracksWeight || repetitions != nil && !capabilities.TracksRepetitions ||
		duration != nil && !capabilities.TracksTime || distance != nil && !capabilities.TracksDistance {
		return ErrMetricNotTracked
	}
	return nil
}

func validateScores(values ...*int16) error {
	for _, value := range values {
		if value != nil && (*value < 1 || *value > 10) {
			return ErrValidation
		}
	}
	return nil
}

func validateOptionalScore(value Optional[int16]) error {
	if value.Set && value.Value != nil && (*value.Value < 1 || *value.Value > 10) {
		return ErrValidation
	}
	return nil
}

func requiredText(value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return "", ErrValidation
	}
	return value, nil
}

func optionalText(value *string, maximum int) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || !utf8.ValidString(normalized) || utf8.RuneCountInString(normalized) > maximum {
		return nil, ErrValidation
	}
	return &normalized, nil
}

func validateOptionalText(value Optional[string], maximum int) error {
	if !value.Set || value.Value == nil {
		return nil
	}
	normalized, err := optionalText(value.Value, maximum)
	if err == nil {
		value.Value = normalized
	}
	return err
}

func validRIR(value float64) bool {
	return finite(value) && value >= 0 && value <= 10 && math.Abs(value*10-math.Round(value*10)) < 1e-9
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func utcTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func workoutPatchHasFields(input PatchInput) bool {
	return input.Name.Set || input.Status.Set || input.ScheduledAt.Set || input.StartedAt.Set || input.CompletedAt.Set ||
		input.Difficulty.Set || input.Energy.Set || input.Mood.Set || input.Comment.Set || input.HasPain.Set || input.Discomfort.Set
}

func setPatchHasFields(input SetPatchInput) bool {
	return input.SetNumber.Set || input.WeightKG.Set || input.Repetitions.Set || input.RIR.Set || input.Warmup.Set || input.Failure.Set ||
		input.DurationSeconds.Set || input.DistanceMeters.Set || input.Note.Set || input.PerformedAt.Set
}
