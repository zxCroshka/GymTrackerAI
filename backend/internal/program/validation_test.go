package program

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testExerciseID = "00000000-0000-4000-8000-000000000001"

func TestNormalizeCreateAcceptsOrderedProgram(t *testing.T) {
	description := "  Набор силы  "
	notes := "  Не спешить  "
	repsMin, repsMax := int16(5), int16(8)
	rir := 2.5
	rest := int32(180)
	input, err := normalizeCreate(CreateInput{
		Name: "  Сила  ", Description: &description,
		Days: []DayInput{{
			Position: 1, Name: "  День 1  ", Notes: &notes,
			Exercises: []DayExerciseInput{{
				ExerciseID: testExerciseID, Position: 1, WorkingSets: 4,
				TargetRepsMin: &repsMin, TargetRepsMax: &repsMax,
				TargetRIR: &rir, RestSeconds: &rest,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("normalizeCreate() error = %v", err)
	}
	if input.Name != "Сила" || input.Description == nil || *input.Description != "Набор силы" ||
		input.Days[0].Name != "День 1" || input.Days[0].Notes == nil || *input.Days[0].Notes != "Не спешить" {
		t.Fatalf("normalized input = %+v", input)
	}
}

func TestProgramListFilterRejectsUnknownAndRepeatedParameters(t *testing.T) {
	for _, target := range []string{
		"/programs?unknown=value",
		"/programs?status=draft&status=active",
		"/programs?include_archived=maybe",
	} {
		request := httptest.NewRequest("GET", target, nil)
		if _, err := programListFilter(request); err == nil {
			t.Fatalf("programListFilter(%q) accepted invalid query", target)
		}
	}
}

func TestValidateCreateRejectsInvalidOrderAndTargets(t *testing.T) {
	repsMin, repsMax := int16(12), int16(8)
	invalidRIR := 2.25
	tests := []struct {
		name string
		item DayExerciseInput
		day  DayInput
	}{
		{name: "day position gap", day: DayInput{Position: 2, Name: "День"}},
		{name: "exercise position gap", item: DayExerciseInput{ExerciseID: testExerciseID, Position: 2, WorkingSets: 3}},
		{name: "repetition range", item: DayExerciseInput{ExerciseID: testExerciseID, Position: 1, WorkingSets: 3, TargetRepsMin: &repsMin, TargetRepsMax: &repsMax}},
		{name: "rir precision", item: DayExerciseInput{ExerciseID: testExerciseID, Position: 1, WorkingSets: 3, TargetRIR: &invalidRIR}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			day := test.day
			if day.Name == "" {
				day = DayInput{Position: 1, Name: "День", Exercises: []DayExerciseInput{test.item}}
			}
			if _, err := normalizeCreate(CreateInput{Name: "Программа", Days: []DayInput{day}}); err != ErrValidation {
				t.Fatalf("normalizeCreate() error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestValidActivationRequiresExercisesInEveryDay(t *testing.T) {
	if validActivation(Program{}) {
		t.Fatal("empty program is activatable")
	}
	if validActivation(Program{Days: []ProgramDay{{Position: 1}}}) {
		t.Fatal("empty day is activatable")
	}
	if !validActivation(Program{Days: []ProgramDay{{Exercises: []ProgramDayExercise{{ExerciseID: testExerciseID}}}}}) {
		t.Fatal("complete program is not activatable")
	}
	if validActivationInput([]DayInput{{Position: 1, Name: "Пустой день"}}) {
		t.Fatal("empty input day is activatable")
	}
	if !validActivationInput([]DayInput{{
		Position: 1, Name: "День",
		Exercises: []DayExerciseInput{{ExerciseID: testExerciseID, Position: 1, WorkingSets: 3}},
	}}) {
		t.Fatal("complete input tree is not activatable")
	}
}

func TestCopyNameIsBounded(t *testing.T) {
	name := strings.Repeat("я", 250)
	copy := copyName(name)
	if len([]rune(copy)) != 200 || !strings.HasSuffix(copy, " (копия)") {
		t.Fatalf("copyName() = %q (%d runes)", copy, len([]rune(copy)))
	}
}

func TestProgramCursorRoundTripAndValidation(t *testing.T) {
	value := Program{ID: testExerciseID, UpdatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.FixedZone("local", 3*60*60))}
	encoded, err := EncodeCursor(value)
	if err != nil {
		t.Fatalf("EncodeCursor() error = %v", err)
	}
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}
	if decoded.ID != value.ID || decoded.UpdatedAt.Location() != time.UTC || !decoded.UpdatedAt.Equal(value.UpdatedAt) {
		t.Fatalf("decoded cursor = %+v", decoded)
	}
	if _, err := DecodeCursor("not-a-cursor"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}
