package exercise

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrValidation      = errors.New("exercise validation failed")
	ErrNotFound        = errors.New("exercise not found")
	ErrNameConflict    = errors.New("exercise name conflict")
	ErrSystemImmutable = errors.New("system exercise is immutable")
	ErrArchived        = errors.New("exercise is archived")
	ErrVersionConflict = errors.New("exercise version conflict")
)

var (
	exerciseTypes  = map[string]struct{}{"strength": {}, "cardio": {}, "stretching": {}, "bodyweight": {}, "isometric": {}}
	equipmentTypes = map[string]struct{}{
		"barbell": {}, "dumbbell": {}, "machine": {}, "cable": {}, "pullup_bar": {},
		"parallel_bars": {}, "bodyweight": {}, "other": {},
	}
	muscleGroups = map[string]struct{}{
		"chest": {}, "back": {}, "quadriceps": {}, "hamstrings": {}, "glutes": {},
		"posterior_chain": {}, "shoulders": {}, "biceps": {}, "triceps": {},
		"calves": {}, "core": {}, "full_body": {}, "cardio": {}, "other": {},
	}
)

func normalizeCreate(input CreateInput) (CreateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = cleanOptional(input.Description)
	input.Instructions = cleanOptional(input.Instructions)
	input.PrimaryMuscleGroup = cleanOptional(input.PrimaryMuscleGroup)
	input.Equipment = cleanOptional(input.Equipment)
	input.MovementPattern = cleanOptional(input.MovementPattern)
	if err := validateInput(input); err != nil {
		return CreateInput{}, err
	}
	return input, nil
}

func validateInput(input CreateInput) error {
	if !validText(input.Name, 1, 200) ||
		!validOptionalText(input.Description, 4000) ||
		!validOptionalText(input.Instructions, 8000) ||
		!validOptionalText(input.MovementPattern, 100) {
		return ErrValidation
	}
	if _, ok := exerciseTypes[input.ExerciseType]; !ok {
		return ErrValidation
	}
	if input.Equipment == nil {
		return ErrValidation
	}
	if _, ok := equipmentTypes[*input.Equipment]; !ok {
		return ErrValidation
	}
	if input.PrimaryMuscleGroup != nil {
		if _, ok := muscleGroups[*input.PrimaryMuscleGroup]; !ok {
			return ErrValidation
		}
	}
	if !input.TracksWeight && !input.TracksRepetitions && !input.TracksTime && !input.TracksDistance {
		return ErrValidation
	}
	return nil
}

func patchHasFields(input PatchInput) bool {
	return input.Name.Set || input.Description.Set || input.Instructions.Set ||
		input.PrimaryMuscleGroup.Set || input.ExerciseType.Set || input.Equipment.Set ||
		input.MovementPattern.Set || input.IsUnilateral.Set || input.TracksWeight.Set ||
		input.TracksRepetitions.Set || input.TracksTime.Set || input.TracksDistance.Set
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}

func validText(value string, minimum, maximum int) bool {
	length := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && value == strings.TrimSpace(value) && length >= minimum && length <= maximum
}

func validOptionalText(value *string, maximum int) bool {
	return value == nil || validText(*value, 1, maximum)
}
