package measurement

import (
	"testing"
	"time"
)

func TestBodyMeasurementValidationBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	minimum := 20.0
	maximum := 700.0
	for _, weight := range []*float64{&minimum, &maximum} {
		if err := validateBody(BodyMeasurement{MeasuredAt: now, WeightKG: weight}); err != nil {
			t.Errorf("weight %v rejected: %v", *weight, err)
		}
	}
	below := 19.999
	if err := validateBody(BodyMeasurement{MeasuredAt: now, WeightKG: &below}); err == nil {
		t.Fatal("weight below lower boundary accepted")
	}
	if err := validateBody(BodyMeasurement{MeasuredAt: now}); err == nil {
		t.Fatal("empty numeric measurement accepted")
	}
}

func TestWellnessValidationEmptyAndBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	zero := int16(0)
	if err := validateWellness(WellnessCreateInput{ObservedAt: now}); err == nil {
		t.Fatal("empty wellness accepted")
	}
	if err := validateWellness(WellnessCreateInput{ObservedAt: now, SleepMinutes: &zero}); err != nil {
		t.Fatalf("zero sleep rejected: %v", err)
	}
	quality := int16(6)
	if err := validateWellness(WellnessCreateInput{ObservedAt: now, SleepQuality: &quality}); err == nil {
		t.Fatal("sleep quality above boundary accepted")
	}
}
