package progress

import (
	"fmt"
	"testing"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

func TestDiscoverRecordsEmpty(t *testing.T) {
	records, err := discoverRecords(nil, "user", time.Now(), func() (string, error) { return "id", nil })
	if err != nil || len(records) != 0 {
		t.Fatalf("empty records = %v, %v", records, err)
	}
}

func TestDiscoverRecordsTracksWeightSpecificRepsAndE1RMBoundary(t *testing.T) {
	at := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	weight100, key100, reps10 := 100.0, "100.000", int16(10)
	weight110, key110, reps16 := 110.0, "110.000", int16(16)
	reps12 := int16(12)
	candidates := []workout.RecordCandidate{{SetID: "s1", ExerciseID: "e", WeightKG: &weight100, WeightKey: &key100, Repetitions: &reps10, AchievedAt: at}, {SetID: "s2", ExerciseID: "e", WeightKG: &weight110, WeightKey: &key110, Repetitions: &reps16, AchievedAt: at.Add(time.Minute)}, {SetID: "s3", ExerciseID: "e", WeightKG: &weight100, WeightKey: &key100, Repetitions: &reps12, AchievedAt: at.Add(2 * time.Minute)}}
	sequence := 0
	records, err := discoverRecords(candidates, "u", at, func() (string, error) { sequence++; return fmt.Sprintf("id-%d", sequence), nil })
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, record := range records {
		counts[record.RecordType]++
		if record.RecordType == "estimated_1rm" && record.SetID == "s2" {
			t.Fatal("e1RM created above 15 repetitions")
		}
	}
	if counts["max_reps"] != 3 {
		t.Fatalf("max-reps discoveries = %d, want three across two weights", counts["max_reps"])
	}
	if counts["estimated_1rm"] != 2 {
		t.Fatalf("e1RM discoveries = %d, want two eligible improvements", counts["estimated_1rm"])
	}
	if counts["max_weight"] != 2 {
		t.Fatalf("max-weight discoveries = %d", counts["max_weight"])
	}
}
