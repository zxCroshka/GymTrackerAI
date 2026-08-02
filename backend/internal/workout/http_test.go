package workout

import (
	"net/http/httptest"
	"testing"
)

const testWorkoutID = "10000000-0000-4000-8000-000000000001"

func TestWorkoutListFilterRejectsInvalidQueries(t *testing.T) {
	for _, target := range []string{
		"/workouts?unknown=x",
		"/workouts?status=completed&status=planned",
		"/workouts?status=unknown",
		"/workouts?limit=0",
		"/workouts?from=not-a-date",
		"/workouts?from=2026-08-03T00:00:00Z&to=2026-08-02T00:00:00Z",
	} {
		request := httptest.NewRequest("GET", target, nil)
		if _, err := workoutListFilter(request, false); err == nil {
			t.Errorf("workoutListFilter(%q) accepted invalid query", target)
		}
	}
}

func TestCSVFilterRejectsPagination(t *testing.T) {
	request := httptest.NewRequest("GET", "/workouts/export.csv?limit=10", nil)
	if _, err := workoutListFilter(request, true); err == nil {
		t.Fatal("CSV filter accepted limit")
	}
}

func TestExpectedWorkoutVersion(t *testing.T) {
	request := httptest.NewRequest("PATCH", "/workout-sets/id", nil)
	recorder := httptest.NewRecorder()
	if _, _, ok := expectedWorkoutVersion(recorder, request); ok || recorder.Code != 428 {
		t.Fatalf("missing If-Match accepted, status=%d", recorder.Code)
	}

	request = httptest.NewRequest("PATCH", "/workout-sets/id", nil)
	request.Header.Set("If-Match", `"workout:`+testWorkoutID+`:7"`)
	recorder = httptest.NewRecorder()
	workoutID, version, ok := expectedWorkoutVersion(recorder, request)
	if !ok || workoutID != testWorkoutID || version != 7 {
		t.Fatalf("expectedWorkoutVersion = %q, %d, %v", workoutID, version, ok)
	}
}

func TestSafeCSVTextPreventsFormulaInjection(t *testing.T) {
	for _, value := range []string{"=SUM(A1:A2)", "+1", "-1", "@cmd", "  =formula"} {
		if got := safeCSVText(value); len(got) == 0 || got[0] != '\'' {
			t.Errorf("safeCSVText(%q) = %q", value, got)
		}
	}
	if got := safeCSVText("normal text"); got != "normal text" {
		t.Fatalf("normal text changed to %q", got)
	}
}
