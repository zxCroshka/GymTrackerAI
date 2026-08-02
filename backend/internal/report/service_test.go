package report

import (
	"testing"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

func TestBuildMetricsEmptyDataIsExplicit(t *testing.T) {
	metrics := buildMetrics(workout.AnalyticsSnapshot{}, workout.AnalyticsSnapshot{}, measurement.WeightTrend{}, measurement.WellnessSummary{}, nil, time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC))
	if metrics.PreviousWeek.VolumeChangePercent != nil {
		t.Fatal("percent change with zero baseline must be null")
	}
	if metrics.NewRecords == nil || metrics.PainMessages == nil || metrics.ExerciseSummaries == nil {
		t.Fatal("empty collections must be JSON arrays")
	}
	if metrics.Coverage.HasWorkoutData || metrics.Coverage.HasWeightData || metrics.Coverage.HasWellnessData {
		t.Fatal("empty coverage was reported as present")
	}
}
func TestBuildMetricsPreviousWeekChange(t *testing.T) {
	current := workout.AnalyticsSnapshot{CompletedWorkouts: 2, VolumeKG: 750}
	previous := workout.AnalyticsSnapshot{CompletedWorkouts: 3, VolumeKG: 1000}
	metrics := buildMetrics(current, previous, measurement.WeightTrend{}, measurement.WellnessSummary{}, nil, time.Now())
	if metrics.PreviousWeek.VolumeChangeKG != -250 || metrics.PreviousWeek.VolumeChangePercent == nil || *metrics.PreviousWeek.VolumeChangePercent != -25 {
		t.Fatalf("comparison=%+v", metrics.PreviousWeek)
	}
}
