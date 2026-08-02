package workout

import (
	"math"
	"testing"
)

func TestEstimated1RMBoundaries(t *testing.T) {
	weight := 100.0
	tests := []struct {
		name   string
		reps   int16
		warmup bool
		want   *float64
	}{
		{name: "zero repetitions", reps: 0, want: nil},
		{name: "one repetition", reps: 1, want: floatPointer(103.333)},
		{name: "fifteen repetitions", reps: 15, want: floatPointer(150)},
		{name: "sixteen repetitions", reps: 16, want: nil},
		{name: "warmup excluded", reps: 8, warmup: true, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Estimated1RM(&weight, &test.reps, test.warmup)
			assertOptionalFloat(t, got, test.want)
		})
	}
}

func TestVolumeExcludesWarmupAndUsesZeroValues(t *testing.T) {
	weight, repetitions := 82.5, int16(10)
	assertOptionalFloat(t, Volume(&weight, &repetitions, false), floatPointer(825))
	if got := Volume(&weight, &repetitions, true); got != nil {
		t.Fatalf("warmup volume = %v, want nil", *got)
	}
	zero := 0.0
	assertOptionalFloat(t, Volume(&zero, &repetitions, false), floatPointer(0))
}

func TestRIRBoundaries(t *testing.T) {
	for _, value := range []float64{0, 0.1, 5.5, 9.9, 10} {
		if !validRIR(value) {
			t.Errorf("validRIR(%v) = false", value)
		}
	}
	for _, value := range []float64{-0.1, 10.1, 2.25, math.NaN(), math.Inf(1)} {
		if validRIR(value) {
			t.Errorf("validRIR(%v) = true", value)
		}
	}
}

func TestCalculateMetricsCountsOnlyPerformedSets(t *testing.T) {
	weight, reps := 100.0, int16(10)
	value := Workout{Exercises: []WorkoutExercise{{Sets: []WorkoutSet{
		{Status: "completed", WeightKG: &weight, Repetitions: &reps},
		{Status: "completed", WeightKG: &weight, Repetitions: &reps, Warmup: true},
		{Status: "planned"},
	}}}}
	calculateMetrics(&value)
	if value.ExerciseCount != 1 || value.SetCount != 2 || value.WorkingSetCount != 1 || value.VolumeKG != 1000 {
		t.Fatalf("metrics = exercises:%d sets:%d working:%d volume:%v", value.ExerciseCount, value.SetCount, value.WorkingSetCount, value.VolumeKG)
	}
}

func assertOptionalFloat(t *testing.T, got, want *float64) {
	t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
		return
	}
	if math.Abs(*got-*want) > 0.0001 {
		t.Fatalf("got %v, want %v", *got, *want)
	}
}

func floatPointer(value float64) *float64 { return &value }
