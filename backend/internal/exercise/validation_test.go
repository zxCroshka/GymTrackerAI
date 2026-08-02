package exercise

import (
	"encoding/json"
	"testing"
)

func TestNormalizeCreateAndTrackingValidation(t *testing.T) {
	equipment, muscle := "dumbbell", "biceps"
	value, err := normalizeCreate(CreateInput{
		Name: "  Подъём гантелей  ", PrimaryMuscleGroup: &muscle,
		ExerciseType: "strength", Equipment: &equipment,
		TracksWeight: true, TracksRepetitions: true,
	})
	if err != nil {
		t.Fatalf("normalizeCreate: %v", err)
	}
	if value.Name != "Подъём гантелей" {
		t.Fatalf("name = %q", value.Name)
	}
	if _, err := normalizeCreate(CreateInput{Name: "Пустое", ExerciseType: "strength", Equipment: &equipment}); err == nil {
		t.Fatal("exercise without a tracking metric accepted")
	}
}

func TestPatchOptionalAndValidation(t *testing.T) {
	var patch PatchInput
	if err := json.Unmarshal([]byte("{\"description\":null,\"tracks_time\":true}"), &patch); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	if !patch.Description.Set || patch.Description.Value != nil {
		t.Fatalf("description = %+v", patch.Description)
	}
	if !patch.TracksTime.Set || patch.TracksTime.Value == nil || !*patch.TracksTime.Value {
		t.Fatalf("tracks_time = %+v", patch.TracksTime)
	}
}

func TestCursorRoundTripAndRejectsTampering(t *testing.T) {
	value := Exercise{ID: "00000000-0000-4000-8000-000000000002", Name: "Жим штанги лёжа"}
	encoded, err := EncodeCursor(value)
	if err != nil {
		t.Fatalf("EncodeCursor: %v", err)
	}
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if decoded.ID != value.ID || decoded.Name != "жим штанги лёжа" {
		t.Fatalf("cursor = %+v", decoded)
	}
	if _, err := DecodeCursor("not-base64"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}
