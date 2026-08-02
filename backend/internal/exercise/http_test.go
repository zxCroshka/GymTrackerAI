package exercise

import (
	"net/http/httptest"
	"testing"
)

func TestListFilterRejectsUnknownAndRepeatedParameters(t *testing.T) {
	for _, target := range []string{
		"/exercises?unknown=value",
		"/exercises?type=strength&type=cardio",
		"/exercises?tracks_time=maybe",
	} {
		request := httptest.NewRequest("GET", target, nil)
		if _, err := listFilter(request); err == nil {
			t.Fatalf("listFilter(%q) accepted invalid query", target)
		}
	}
}

func TestExpectedVersionRequiresExactExerciseETag(t *testing.T) {
	const exerciseID = "00000000-0000-4000-8000-000000000001"
	request := httptest.NewRequest("PATCH", "/exercises/"+exerciseID, nil)
	recorder := httptest.NewRecorder()
	if _, ok := expectedVersion(recorder, request, "exercise", exerciseID); ok || recorder.Code != 428 {
		t.Fatalf("missing If-Match accepted, status=%d", recorder.Code)
	}

	request = httptest.NewRequest("PATCH", "/exercises/"+exerciseID, nil)
	request.Header.Set("If-Match", `"exercise:`+exerciseID+`:7"`)
	recorder = httptest.NewRecorder()
	version, ok := expectedVersion(recorder, request, "exercise", exerciseID)
	if !ok || version != 7 {
		t.Fatalf("expectedVersion() = %d, %v", version, ok)
	}
}
