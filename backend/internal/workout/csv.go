package workout

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const maxExportRows = 100000

var csvHeader = []string{
	"workout_id", "workout_name", "status", "source_program_id", "source_program_day_id", "event_at",
	"scheduled_at", "started_at", "completed_at", "cancelled_at", "difficulty", "energy", "mood",
	"comment", "has_pain", "discomfort", "workout_volume_kg",
	"exercise_position", "workout_exercise_id", "exercise_id", "exercise_name",
	"set_number", "set_status", "weight_kg", "reps", "rir", "warmup", "failure", "duration_seconds",
	"distance_meters", "note", "performed_at", "set_volume_kg", "estimated_1rm_kg",
}

type CSVRow struct {
	WorkoutID, WorkoutName, Status                   string
	SourceProgramID, SourceProgramDayID              *string
	EventAt                                          time.Time
	ScheduledAt, StartedAt, CompletedAt, CancelledAt *time.Time
	Difficulty, Energy, Mood                         *int16
	Comment                                          *string
	HasPain                                          *bool
	Discomfort                                       *string
	WorkoutVolumeKG                                  float64
	ExercisePosition                                 *int16
	WorkoutExerciseID, ExerciseID, ExerciseName      *string
	SetNumber, Repetitions                           *int16
	SetStatus                                        *string
	WeightKG, RIR, DistanceMeters                    *float64
	Warmup, Failure                                  *bool
	DurationSeconds                                  *int32
	Note                                             *string
	PerformedAt                                      *time.Time
}

func (s *Service) ExportCSV(ctx context.Context, actorID string, filter ListFilter, destination io.Writer) error {
	filter.Limit, filter.Cursor = 50, nil
	if err := validateListFilter(filter); err != nil {
		return err
	}
	rows, err := s.repository.ExportRows(ctx, actorID, filter, maxExportRows)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(destination)
	if err := writer.Write(csvHeader); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, row := range rows {
		if err := writer.Write(row.values()); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV: %w", err)
	}
	return nil
}

func (r *Repository) ExportRows(ctx context.Context, actorID string, filter ListFilter, maximum int) ([]CSVRow, error) {
	args := []any{actorID}
	conditions := []string{"w.user_id = $1"}
	conditions, args = appendFilterConditions(conditions, args, filter, true)
	exerciseJoin := `LEFT JOIN workout_exercises AS item
		ON item.workout_id = w.id AND item.user_id = w.user_id`
	if filter.ExerciseID != "" {
		args = append(args, filter.ExerciseID)
		exerciseJoin = fmt.Sprintf(`JOIN workout_exercises AS item
			ON item.workout_id = w.id AND item.user_id = w.user_id AND item.exercise_id = $%d`, len(args))
	}
	args = append(args, maximum+1)
	statement := `
		SELECT w.id, w.name, w.status,
		       (SELECT day.program_id FROM program_days AS day WHERE day.id = w.source_program_day_id),
		       w.source_program_day_id, COALESCE(w.started_at, w.scheduled_at, w.created_at),
		       w.scheduled_at, w.started_at, w.completed_at, w.cancelled_at,
		       w.difficulty, w.energy, w.mood, w.notes, w.has_pain, w.discomfort,
		       COALESCE(stats.volume_kg, 0)::double precision,
		       item.position, item.id, item.exercise_id, item.exercise_name_snapshot,
		       set.position, set.status, set.weight_kg, set.reps, set.rir,
		       CASE WHEN set.id IS NULL THEN NULL ELSE set.set_type = 'warmup' END,
		       CASE WHEN set.id IS NULL THEN NULL ELSE set.set_type = 'failure' END,
		       set.duration_seconds, set.distance_meters, set.notes, set.completed_at
		FROM workouts AS w
		LEFT JOIN LATERAL (
			SELECT sum(metric.weight_kg * metric.reps) FILTER (
				WHERE metric.status = 'completed' AND metric.set_type <> 'warmup'
				  AND metric.weight_kg IS NOT NULL AND metric.reps IS NOT NULL
			) AS volume_kg
			FROM workout_exercises AS metric_item
			JOIN workout_sets AS metric ON metric.workout_exercise_id = metric_item.id
			WHERE metric_item.workout_id = w.id AND metric_item.user_id = w.user_id
		) AS stats ON true
		` + exerciseJoin + `
		LEFT JOIN workout_sets AS set ON set.workout_exercise_id = item.id AND set.user_id = item.user_id
		WHERE ` + strings.Join(conditions, " AND ") +
		fmt.Sprintf(` ORDER BY COALESCE(w.started_at, w.scheduled_at, w.created_at), w.id,
			item.position NULLS FIRST, set.position NULLS FIRST LIMIT $%d`, len(args))
	rows, err := r.pool.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("select workout CSV rows: %w", err)
	}
	defer rows.Close()
	result := []CSVRow{}
	for rows.Next() {
		var row CSVRow
		if err := rows.Scan(
			&row.WorkoutID, &row.WorkoutName, &row.Status, &row.SourceProgramID,
			&row.SourceProgramDayID, &row.EventAt, &row.ScheduledAt, &row.StartedAt, &row.CompletedAt,
			&row.CancelledAt, &row.Difficulty, &row.Energy, &row.Mood, &row.Comment,
			&row.HasPain, &row.Discomfort, &row.WorkoutVolumeKG, &row.ExercisePosition,
			&row.WorkoutExerciseID, &row.ExerciseID, &row.ExerciseName, &row.SetNumber, &row.SetStatus,
			&row.WeightKG, &row.Repetitions, &row.RIR, &row.Warmup, &row.Failure,
			&row.DurationSeconds, &row.DistanceMeters, &row.Note, &row.PerformedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workout CSV row: %w", err)
		}
		result = append(result, row)
		if len(result) > maximum {
			return nil, ErrExportTooLarge
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workout CSV rows: %w", err)
	}
	return result, nil
}

func (row CSVRow) values() []string {
	setVolume := Volume(row.WeightKG, row.Repetitions, boolValue(row.Warmup))
	estimated := Estimated1RM(row.WeightKG, row.Repetitions, boolValue(row.Warmup))
	return []string{
		row.WorkoutID, safeCSVText(row.WorkoutName), row.Status, stringValue(row.SourceProgramID), stringValue(row.SourceProgramDayID), row.EventAt.UTC().Format(time.RFC3339Nano),
		timeValue(row.ScheduledAt), timeValue(row.StartedAt), timeValue(row.CompletedAt), timeValue(row.CancelledAt),
		int16Value(row.Difficulty), int16Value(row.Energy), int16Value(row.Mood), safeCSVPointer(row.Comment), boolPointerValue(row.HasPain), safeCSVPointer(row.Discomfort),
		floatValue(&row.WorkoutVolumeKG), int16Value(row.ExercisePosition), stringValue(row.WorkoutExerciseID), stringValue(row.ExerciseID), safeCSVPointer(row.ExerciseName),
		int16Value(row.SetNumber), stringValue(row.SetStatus), floatValue(row.WeightKG), int16Value(row.Repetitions), floatValue(row.RIR), boolPointerValue(row.Warmup), boolPointerValue(row.Failure),
		int32Value(row.DurationSeconds), floatValue(row.DistanceMeters), safeCSVPointer(row.Note), timeValue(row.PerformedAt), floatValue(setVolume), floatValue(estimated),
	}
}

func safeCSVText(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func safeCSVPointer(value *string) string {
	if value == nil {
		return ""
	}
	return safeCSVText(*value)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func timeValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func floatValue(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(roundMetric(*value), 'f', -1, 64)
}

func int16Value(value *int16) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(*value), 10)
}

func int32Value(value *int32) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(*value), 10)
}

func boolPointerValue(value *bool) string {
	if value == nil {
		return ""
	}
	return strconv.FormatBool(*value)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}
