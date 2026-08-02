package measurement

import (
	"math"
	"strings"
	"time"
)

func validateBody(value BodyMeasurement) error {
	if value.MeasuredAt.IsZero() || !validNote(value.Notes) {
		return ErrValidation
	}
	values := []*float64{value.WeightKG, value.ChestCM, value.WaistCM, value.HipsCM,
		value.NeckCM, value.LeftUpperArmCM, value.RightUpperArmCM, value.LeftThighCM,
		value.RightThighCM, value.BodyFatPercent}
	present := false
	for _, current := range values {
		if current != nil {
			present = true
			if math.IsNaN(*current) || math.IsInf(*current, 0) {
				return ErrValidation
			}
		}
	}
	if !present || !between(value.WeightKG, 20, 700) ||
		!between(value.ChestCM, 5, 400) || !between(value.WaistCM, 5, 400) ||
		!between(value.HipsCM, 5, 400) || !between(value.NeckCM, 5, 200) ||
		!between(value.LeftUpperArmCM, 5, 200) || !between(value.RightUpperArmCM, 5, 200) ||
		!between(value.LeftThighCM, 5, 250) || !between(value.RightThighCM, 5, 250) ||
		!between(value.BodyFatPercent, 0, 100) {
		return ErrValidation
	}
	return nil
}

func validateWellness(value WellnessCreateInput) error {
	if value.ObservedAt.IsZero() || !validNote(value.Notes) {
		return ErrValidation
	}
	present := value.SleepMinutes != nil || value.SleepQuality != nil || value.Energy != nil ||
		value.Steps != nil || value.CaloriesKcal != nil || value.ProteinG != nil || value.FatG != nil ||
		value.CarbsG != nil || value.Notes != nil && strings.TrimSpace(*value.Notes) != ""
	if !present || !betweenInt16(value.SleepMinutes, 0, 1440) ||
		!betweenInt16(value.SleepQuality, 1, 5) || !betweenInt16(value.Energy, 1, 5) ||
		!betweenInt32(value.Steps, 0, 1000000) || !between(value.CaloriesKcal, 0, 50000) ||
		!between(value.ProteinG, 0, 5000) || !between(value.FatG, 0, 5000) ||
		!between(value.CarbsG, 0, 5000) {
		return ErrValidation
	}
	return nil
}

func validateFilter(filter ListFilter) error {
	if filter.Limit < 1 || filter.Limit > 100 || filter.From != nil && filter.To != nil && !filter.From.Before(*filter.To) {
		return ErrValidation
	}
	if filter.From != nil && filter.To != nil && filter.To.Sub(*filter.From) > 2*365*24*time.Hour {
		return ErrValidation
	}
	return nil
}

func validNote(value *string) bool {
	if value == nil {
		return true
	}
	trimmed := strings.TrimSpace(*value)
	return len(trimmed) > 0 && len(*value) <= 4000
}

func cleanNote(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func between(value *float64, minimum, maximum float64) bool {
	return value == nil || !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= minimum && *value <= maximum
}

func betweenInt16(value *int16, minimum, maximum int16) bool {
	return value == nil || *value >= minimum && *value <= maximum
}

func betweenInt32(value *int32, minimum, maximum int32) bool {
	return value == nil || *value >= minimum && *value <= maximum
}

func bodyPatchHasFields(value BodyPatchInput) bool {
	return value.MeasuredAt.Set || value.WeightKG.Set || value.ChestCM.Set || value.WaistCM.Set ||
		value.HipsCM.Set || value.NeckCM.Set || value.LeftUpperArmCM.Set || value.RightUpperArmCM.Set ||
		value.LeftThighCM.Set || value.RightThighCM.Set || value.BodyFatPercent.Set || value.Notes.Set
}
