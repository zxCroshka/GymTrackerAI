package progress

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type progressQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r *Repository) ReplaceUser(ctx context.Context, tx pgx.Tx, userID string, records []recordWrite) error {
	type existingRecord struct {
		id        string
		createdAt time.Time
	}
	existing := map[string]existingRecord{}
	rows, err := tx.Query(ctx, `SELECT id,workout_set_id,record_type,created_at FROM personal_records WHERE user_id=$1`, userID)
	if err != nil {
		return fmt.Errorf("load existing personal record projection: %w", err)
	}
	for rows.Next() {
		var id, setID, kind string
		var createdAt time.Time
		if err := rows.Scan(&id, &setID, &kind, &createdAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan existing personal record: %w", err)
		}
		existing[setID+"\x00"+kind] = existingRecord{id: id, createdAt: createdAt.UTC()}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate existing personal records: %w", err)
	}
	rows.Close()
	for index := range records {
		if old, ok := existing[records[index].SetID+"\x00"+records[index].RecordType]; ok {
			records[index].ID = old.id
			records[index].CreatedAt = old.createdAt
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM personal_records WHERE user_id=$1`, userID); err != nil {
		return fmt.Errorf("clear personal record projection: %w", err)
	}
	for _, value := range records {
		if _, err := tx.Exec(ctx, `INSERT INTO personal_records(id,user_id,exercise_id,workout_set_id,record_type,value,calculation_version,formula,achieved_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, value.ID, userID, value.ExerciseID, value.SetID, value.RecordType, value.Value, CalculationVersion, value.Formula, value.AchievedAt.UTC(), value.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert personal record projection: %w", err)
		}
	}
	return nil
}

func (r *Repository) Current(ctx context.Context, actorID string, filter RecordFilter) ([]PersonalRecord, error) {
	return r.current(ctx, r.pool, actorID, filter)
}
func (r *Repository) current(ctx context.Context, query progressQuery, actorID string, filter RecordFilter) ([]PersonalRecord, error) {
	args := []any{actorID}
	conditions := []string{"ranked.rank=1"}
	if filter.ExerciseID != "" {
		args = append(args, filter.ExerciseID)
		conditions = append(conditions, fmt.Sprintf("ranked.exercise_id=$%d", len(args)))
	}
	if filter.RecordType != "" {
		args = append(args, filter.RecordType)
		conditions = append(conditions, fmt.Sprintf("ranked.record_type=$%d", len(args)))
	}
	if filter.From != nil {
		args = append(args, filter.From.UTC())
		conditions = append(conditions, fmt.Sprintf("ranked.achieved_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, filter.To.UTC())
		conditions = append(conditions, fmt.Sprintf("ranked.achieved_at < $%d", len(args)))
	}
	args = append(args, filter.Limit)
	statement := `WITH ranked AS (
	 SELECT pr.id,pr.exercise_id,e.name,w.id AS workout_id,pr.workout_set_id,pr.record_type,
	        pr.value::double precision,s.weight_kg::double precision,pr.calculation_version,
	        pr.formula,pr.achieved_at,
	        row_number() OVER(PARTITION BY pr.exercise_id,pr.record_type,
	          CASE WHEN pr.record_type='max_reps' THEN s.weight_kg::text ELSE '' END
	          ORDER BY pr.value DESC,pr.achieved_at DESC,pr.id DESC) AS rank
	 FROM personal_records pr JOIN workout_sets s ON s.id=pr.workout_set_id AND s.user_id=pr.user_id
	 JOIN workout_exercises item ON item.id=s.workout_exercise_id AND item.user_id=s.user_id
	 JOIN workouts w ON w.id=item.workout_id AND w.user_id=item.user_id
	 JOIN exercises e ON e.id=pr.exercise_id WHERE pr.user_id=$1
	) SELECT id,exercise_id,name,workout_id,workout_set_id,record_type,value,
	 CASE record_type WHEN 'max_reps' THEN 'repetitions' WHEN 'max_set_volume' THEN 'kg_repetitions' ELSE 'kg' END,
	 CASE WHEN record_type='max_reps' THEN weight_kg ELSE NULL END,
	 calculation_version,formula,achieved_at FROM ranked WHERE ` + strings.Join(conditions, " AND ") + fmt.Sprintf(" ORDER BY achieved_at DESC,id DESC LIMIT $%d", len(args))
	rows, err := query.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("query current personal records: %w", err)
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (r *Repository) Achievements(ctx context.Context, query progressQuery, actorID string, from, to time.Time) ([]PersonalRecord, error) {
	rows, err := query.Query(ctx, `
	SELECT pr.id,pr.exercise_id,e.name,w.id,pr.workout_set_id,pr.record_type,pr.value::double precision,
	 CASE pr.record_type WHEN 'max_reps' THEN 'repetitions' WHEN 'max_set_volume' THEN 'kg_repetitions' ELSE 'kg' END,
	 CASE WHEN pr.record_type='max_reps' THEN s.weight_kg::double precision ELSE NULL END,
	 pr.calculation_version,pr.formula,pr.achieved_at
	FROM personal_records pr JOIN workout_sets s ON s.id=pr.workout_set_id AND s.user_id=pr.user_id JOIN workout_exercises item ON item.id=s.workout_exercise_id AND item.user_id=s.user_id JOIN workouts w ON w.id=item.workout_id AND w.user_id=item.user_id JOIN exercises e ON e.id=pr.exercise_id
	WHERE pr.user_id=$1 AND pr.achieved_at >= $2 AND pr.achieved_at < $3 ORDER BY pr.achieved_at,pr.id`, actorID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query new personal records: %w", err)
	}
	defer rows.Close()
	return scanRecords(rows)
}

func scanRecords(rows pgx.Rows) ([]PersonalRecord, error) {
	result := []PersonalRecord{}
	for rows.Next() {
		var value PersonalRecord
		if err := rows.Scan(&value.ID, &value.ExerciseID, &value.ExerciseName, &value.WorkoutID, &value.WorkoutSetID, &value.RecordType, &value.Value, &value.Unit, &value.WeightKG, &value.CalculationVersion, &value.Formula, &value.AchievedAt); err != nil {
			return nil, fmt.Errorf("scan personal record: %w", err)
		}
		value.Value = math.Round(value.Value*1000) / 1000
		if value.WeightKG != nil {
			v := math.Round(*value.WeightKG*1000) / 1000
			value.WeightKG = &v
		}
		value.AchievedAt = value.AchievedAt.UTC()
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate personal records: %w", err)
	}
	return result, nil
}
