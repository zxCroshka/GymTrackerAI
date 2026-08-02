package calendartime

import (
	"testing"
	"time"
)

func TestWeekContainingPreservesDSTWeekLength(t *testing.T) {
	start, end, err := WeekContaining(time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC), "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	if duration := end.Sub(start); duration != 167*time.Hour {
		t.Fatalf("spring DST week duration = %s, want 167h", duration)
	}
}

func TestWeekContainingPreservesAutumnDSTWeekLength(t *testing.T) {
	start, end, err := WeekContaining(time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC), "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	if duration := end.Sub(start); duration != 169*time.Hour {
		t.Fatalf("autumn DST week duration = %s, want 169h", duration)
	}
}

func TestDayStartRejectsInvalidTimezone(t *testing.T) {
	if _, err := DayStart(time.Now(), "not/a-zone"); err == nil {
		t.Fatal("invalid timezone accepted")
	}
}
