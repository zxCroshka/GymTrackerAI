package progress

import (
	"testing"
	"time"
)

func TestTrainingStreakUsesConsecutiveLocalWeeks(t *testing.T) {
	start := time.Date(2026, 7, 26, 21, 0, 0, 0, time.UTC)
	keys := []string{"2026-07-27", "2026-07-20", "2026-07-06"}
	if got := trainingStreak(keys, start, "Europe/Moscow"); got != 2 {
		t.Fatalf("streak=%d,want 2", got)
	}
}
func TestTrainingStreakMayEndInPreviousWeek(t *testing.T) {
	start := time.Date(2026, 7, 26, 21, 0, 0, 0, time.UTC)
	keys := []string{"2026-07-20", "2026-07-13"}
	if got := trainingStreak(keys, start, "Europe/Moscow"); got != 2 {
		t.Fatalf("streak=%d,want 2", got)
	}
}
