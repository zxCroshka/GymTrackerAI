package program

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

var (
	ErrValidation          = errors.New("program validation failed")
	ErrNotFound            = errors.New("program not found")
	ErrArchived            = errors.New("program is archived")
	ErrVersionConflict     = errors.New("program version conflict")
	ErrExerciseUnavailable = errors.New("exercise is unavailable")
	ErrNotActivatable      = errors.New("program is not activatable")
)

func normalizeCreate(input CreateInput) (CreateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = cleanOptional(input.Description)
	input.Goal = cleanOptional(input.Goal)
	for dayIndex := range input.Days {
		input.Days[dayIndex].Name = strings.TrimSpace(input.Days[dayIndex].Name)
		input.Days[dayIndex].Notes = cleanOptional(input.Days[dayIndex].Notes)
		for exerciseIndex := range input.Days[dayIndex].Exercises {
			item := &input.Days[dayIndex].Exercises[exerciseIndex]
			item.Notes = cleanOptional(item.Notes)
		}
	}
	if err := validateCreate(input); err != nil {
		return CreateInput{}, err
	}
	return input, nil
}

func validateCreate(input CreateInput) error {
	if !validText(input.Name, 1, 200) || !validOptionalText(input.Description, 4000) ||
		!validOptionalText(input.Goal, 2000) || len(input.Days) > 14 {
		return ErrValidation
	}
	for dayIndex, day := range input.Days {
		if day.Position != int16(dayIndex+1) || !validText(day.Name, 1, 200) ||
			!validOptionalText(day.Notes, 4000) || len(day.Exercises) > 50 {
			return ErrValidation
		}
		for exerciseIndex, item := range day.Exercises {
			if item.Position != int16(exerciseIndex+1) || !id.ValidUUID(item.ExerciseID) ||
				item.WorkingSets < 1 || item.WorkingSets > 100 ||
				!validOptionalText(item.Notes, 4000) {
				return ErrValidation
			}
			if item.TargetRepsMin != nil && *item.TargetRepsMin < 0 ||
				item.TargetRepsMax != nil && *item.TargetRepsMax < 0 ||
				item.TargetRepsMin != nil && item.TargetRepsMax != nil && *item.TargetRepsMax < *item.TargetRepsMin {
				return ErrValidation
			}
			if item.TargetRIR != nil && (math.IsNaN(*item.TargetRIR) || math.IsInf(*item.TargetRIR, 0) ||
				*item.TargetRIR < 0 || *item.TargetRIR > 10 ||
				math.Abs(*item.TargetRIR*10-math.Round(*item.TargetRIR*10)) > 1e-9) {
				return ErrValidation
			}
			if item.RestSeconds != nil && (*item.RestSeconds < 0 || *item.RestSeconds > 86400) {
				return ErrValidation
			}
		}
	}
	return nil
}

func patchHasFields(input PatchInput) bool {
	return input.Name.Set || input.Description.Set || input.Goal.Set || input.Days.Set
}

func validActivation(value Program) bool {
	if len(value.Days) == 0 {
		return false
	}
	for _, day := range value.Days {
		if len(day.Exercises) == 0 {
			return false
		}
	}
	return true
}

func validActivationInput(days []DayInput) bool {
	if len(days) == 0 {
		return false
	}
	for _, day := range days {
		if len(day.Exercises) == 0 {
			return false
		}
	}
	return true
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
