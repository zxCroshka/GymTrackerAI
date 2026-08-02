package user

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPatchInputTracksAbsentNullAndValue(t *testing.T) {
	var input PatchInput
	if err := json.Unmarshal([]byte("{\"name\":null,\"goal\":\"strength\"}"), &input); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !input.Name.Set || input.Name.Value != nil {
		t.Fatalf("name optional = %+v", input.Name)
	}
	if !input.Goal.Set || input.Goal.Value == nil || *input.Goal.Value != "strength" {
		t.Fatalf("goal optional = %+v", input.Goal)
	}
	if input.Sex.Set {
		t.Fatal("absent field marked as set")
	}
}

func TestValidateProfileEnumsRangesAndTimezone(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	goal, level, sex := "recomposition", "intermediate", "male"
	height, sleep := 170.0, 8.0
	frequency := int16(4)
	value := databaseProfile{
		Goal: &goal, ExperienceLevel: &level, Sex: &sex, HeightCM: &height,
		SleepHoursAverage: &sleep, TrainingFrequency: &frequency,
		Timezone: "Europe/Moscow", UnitSystem: "metric",
	}
	if err := validateProfile(value, now); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
	invalidGoal := "bulk"
	value.Goal = &invalidGoal
	if err := validateProfile(value, now); err == nil {
		t.Fatal("unsupported goal accepted")
	}
}

func TestValidateImportRejectsInvalidNotesAndMeasurements(t *testing.T) {
	empty := []string{" "}
	if err := validateImport(ImportInput{Notes: &empty}); err == nil {
		t.Fatal("blank note accepted")
	}
	invalid := 4.9
	if err := validateImport(ImportInput{Measurements: &MeasurementsImport{BicepsCM: &invalid}}); err == nil {
		t.Fatal("out-of-range measurement accepted")
	}
	if err := validateImport(ImportInput{Measurements: &MeasurementsImport{}}); err == nil {
		t.Fatal("empty measurements object accepted")
	}
}
