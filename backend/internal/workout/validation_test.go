package workout

import (
	"errors"
	"testing"
	"time"
)

func TestSetValidationBoundaries(t *testing.T) {
	weightZero, weightNegative := 0.0, -0.001
	repsZero, repsNegative := int16(0), int16(-1)
	durationZero := int32(0)
	distanceZero := 0.0
	if err := validateActualValues(&weightZero, &repsZero, nil, &durationZero, &distanceZero); err != nil {
		t.Fatalf("zero boundaries rejected: %v", err)
	}
	if err := validateActualValues(&weightNegative, nil, nil, nil, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("negative weight error = %v", err)
	}
	if err := validateActualValues(nil, &repsNegative, nil, nil, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("negative repetitions error = %v", err)
	}
}

func TestNormalizeSetCreateChecksCapabilitiesAndFlags(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	weight, reps := 50.0, int16(8)
	strength := WorkoutExercise{TracksWeight: true, TracksRepetitions: true}
	value, err := normalizeSetCreate(SetCreateInput{WeightKG: &weight, Repetitions: &reps}, strength, now)
	if err != nil || value.PerformedAt == nil || !value.PerformedAt.Equal(now) {
		t.Fatalf("normalize strength set = %+v, %v", value, err)
	}
	duration := int32(60)
	if _, err := normalizeSetCreate(SetCreateInput{DurationSeconds: &duration}, strength, now); !errors.Is(err, ErrMetricNotTracked) {
		t.Fatalf("unsupported metric error = %v", err)
	}
	if _, err := normalizeSetCreate(SetCreateInput{WeightKG: &weight, Warmup: true, Failure: true}, strength, now); !errors.Is(err, ErrValidation) {
		t.Fatalf("warmup+failure error = %v", err)
	}
}

func TestCompletedWorkoutRequiresEndTime(t *testing.T) {
	started := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	if err := validateWorkoutTimes(Workout{Status: "completed", StartedAt: &started}, nil, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing completed_at error = %v", err)
	}
	ended := started.Add(time.Hour)
	performedAfter := ended.Add(time.Second)
	if err := validateWorkoutTimes(Workout{Status: "completed", StartedAt: &started, CompletedAt: &ended}, nil, &performedAfter); !errors.Is(err, ErrValidation) {
		t.Fatalf("late set error = %v", err)
	}
}
