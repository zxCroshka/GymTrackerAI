package calendartime

import (
	"errors"
	"time"
	_ "time/tzdata"
)

var ErrInvalidTimezone = errors.New("invalid IANA timezone")

// DayStart returns the first real instant belonging to observed's civil date
// in timezone. Searching by UTC instant also handles zones whose civil day did
// not begin at 00:00 because of a historical offset transition.
func DayStart(observed time.Time, timezone string) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, ErrInvalidTimezone
	}
	local := observed.In(location)
	return firstInstantOfDate(local.Year(), local.Month(), local.Day(), location), nil
}

// WeekContaining returns the Monday-based profile-local week as a half-open
// pair of UTC instants. Its duration may be 167 or 169 hours around DST.
func WeekContaining(instant time.Time, timezone string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidTimezone
	}
	local := instant.In(location)
	daysSinceMonday := (int(local.Weekday()) + 6) % 7
	mondayNoon := time.Date(local.Year(), local.Month(), local.Day()-daysSinceMonday, 12, 0, 0, 0, location)
	start := firstInstantOfDate(mondayNoon.Year(), mondayNoon.Month(), mondayNoon.Day(), location)
	nextMondayNoon := mondayNoon.AddDate(0, 0, 7)
	end := firstInstantOfDate(nextMondayNoon.Year(), nextMondayNoon.Month(), nextMondayNoon.Day(), location)
	return start.UTC(), end.UTC(), nil
}

// WeekStartKey is the YYYY-MM-DD key of the Monday starting instant's local
// civil date and is suitable for comparing distinct activity weeks.
func WeekStartKey(instant time.Time, timezone string) (string, error) {
	start, _, err := WeekContaining(instant, timezone)
	if err != nil {
		return "", err
	}
	location, _ := time.LoadLocation(timezone)
	return start.In(location).Format(time.DateOnly), nil
}

func firstInstantOfDate(year int, month time.Month, day int, location *time.Location) time.Time {
	// The requested date is known to exist because it came from an observed
	// instant. Four UTC days comfortably bracket every IANA offset transition.
	target := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
	low := target.Add(-48 * time.Hour).Unix()
	high := target.Add(48 * time.Hour).Unix()
	for low < high {
		middle := low + (high-low)/2
		value := time.Unix(middle, 0).In(location)
		if compareDate(value.Year(), value.Month(), value.Day(), year, month, day) < 0 {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return time.Unix(low, 0).UTC()
}

func compareDate(leftYear int, leftMonth time.Month, leftDay, rightYear int, rightMonth time.Month, rightDay int) int {
	if leftYear != rightYear {
		return leftYear - rightYear
	}
	if leftMonth != rightMonth {
		return int(leftMonth - rightMonth)
	}
	return leftDay - rightDay
}
