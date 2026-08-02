package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const reportColumns = `id,period_start_at,period_end_at,timezone_at_generation,revision,is_current,supersedes_report_id,status,metrics_schema_version,metrics,input_data_through_at,ai_insight_status,generated_at,version,created_at,updated_at`

type reportScanner interface{ Scan(...any) error }

func (r *Repository) CurrentForPeriod(ctx context.Context, tx pgx.Tx, actorID string, start, end time.Time) (*WeeklyReport, error) {
	value, err := scanReport(tx.QueryRow(ctx, `SELECT `+reportColumns+` FROM weekly_reports WHERE user_id=$1 AND period_start_at=$2 AND period_end_at=$3 AND is_current FOR UPDATE`, actorID, start.UTC(), end.UTC()))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *Repository) InsertReady(ctx context.Context, tx pgx.Tx, actorID string, value WeeklyReport) error {
	metrics, err := json.Marshal(value.Metrics)
	if err != nil {
		return fmt.Errorf("marshal weekly metrics: %w", err)
	}
	if value.SupersedesReportID != nil {
		if _, err := tx.Exec(ctx, `UPDATE weekly_reports SET is_current=false,version=version+1,updated_at=$3 WHERE id=$1 AND user_id=$2 AND is_current`, *value.SupersedesReportID, actorID, value.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("retire current weekly report: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `
	INSERT INTO weekly_reports(id,user_id,period_start_at,period_end_at,timezone_at_generation,revision,is_current,supersedes_report_id,status,metrics_schema_version,metrics,input_data_through_at,ai_insight_status,attempt_count,retryable,generated_at,version,created_at,updated_at)
	VALUES($1,$2,$3,$4,$5,$6,true,$7,'ready',$8,$9::jsonb,$10,'not_requested',0,false,$11,1,$11,$11)`, value.ID, actorID, value.PeriodStartAt.UTC(), value.PeriodEndAt.UTC(), value.Timezone, value.Revision, value.SupersedesReportID, value.MetricsSchemaVersion, string(metrics), value.InputDataThroughAt, value.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert ready weekly report: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, actorID, reportID string) (WeeklyReport, error) {
	return scanReport(r.pool.QueryRow(ctx, `SELECT `+reportColumns+` FROM weekly_reports WHERE id=$1 AND user_id=$2`, reportID, actorID))
}

func (r *Repository) List(ctx context.Context, actorID string, filter ListFilter) ([]WeeklyReport, error) {
	args := []any{actorID}
	conditions := []string{"user_id=$1"}
	if !filter.IncludeRevisions {
		conditions = append(conditions, "is_current")
	}
	if filter.From != nil {
		args = append(args, filter.From.UTC())
		conditions = append(conditions, fmt.Sprintf("period_start_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, filter.To.UTC())
		conditions = append(conditions, fmt.Sprintf("period_start_at < $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status=$%d", len(args)))
	}
	args = append(args, filter.Limit)
	rows, err := r.pool.Query(ctx, `SELECT `+reportColumns+` FROM weekly_reports WHERE `+strings.Join(conditions, " AND ")+fmt.Sprintf(" ORDER BY period_start_at DESC,revision DESC,id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list weekly reports: %w", err)
	}
	defer rows.Close()
	result := []WeeklyReport{}
	for rows.Next() {
		value, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func scanReport(row reportScanner) (WeeklyReport, error) {
	var value WeeklyReport
	var raw []byte
	err := row.Scan(&value.ID, &value.PeriodStartAt, &value.PeriodEndAt, &value.Timezone, &value.Revision, &value.IsCurrent, &value.SupersedesReportID, &value.Status, &value.MetricsSchemaVersion, &raw, &value.InputDataThroughAt, &value.AIInsightStatus, &value.GeneratedAt, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WeeklyReport{}, ErrNotFound
	}
	if err != nil {
		return WeeklyReport{}, fmt.Errorf("scan weekly report: %w", err)
	}
	if raw != nil {
		var metrics WeeklyMetrics
		if err := json.Unmarshal(raw, &metrics); err != nil {
			return WeeklyReport{}, fmt.Errorf("decode stored weekly metrics: %w", err)
		}
		value.Metrics = &metrics
	}
	value.PeriodStartAt = value.PeriodStartAt.UTC()
	value.PeriodEndAt = value.PeriodEndAt.UTC()
	value.InputDataThroughAt = utc(value.InputDataThroughAt)
	value.GeneratedAt = utc(value.GeneratedAt)
	value.CreatedAt = value.CreatedAt.UTC()
	value.UpdatedAt = value.UpdatedAt.UTC()
	return value, nil
}
func utc(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}
