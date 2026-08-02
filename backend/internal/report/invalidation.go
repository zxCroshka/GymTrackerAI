package report

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SourceInvalidator serializes report generation with report-source writes for
// one user, then marks only current ready artifacts whose half-open interval
// contains an affected old or new event instant.
type SourceInvalidator struct{}

func NewSourceInvalidator() *SourceInvalidator { return &SourceInvalidator{} }

func (i *SourceInvalidator) LockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	var lockedID string
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&lockedID); err != nil {
		return fmt.Errorf("lock report source user: %w", err)
	}
	return nil
}

func (i *SourceInvalidator) MarkPeriodsStale(ctx context.Context, tx pgx.Tx, userID string, instants []time.Time, now time.Time) error {
	if len(instants) == 0 {
		return nil
	}
	values := make([]time.Time, 0, len(instants))
	for _, instant := range instants {
		if !instant.IsZero() {
			values = append(values, instant.UTC())
		}
	}
	if len(values) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE weekly_reports AS report
		SET status = 'stale', version = version + 1, updated_at = $3
		WHERE report.user_id = $1 AND report.is_current AND report.status = 'ready'
		  AND EXISTS (
			SELECT 1 FROM unnest($2::timestamptz[]) AS affected(at)
			WHERE affected.at >= report.period_start_at AND affected.at < report.period_end_at
		  )`, userID, values, now.UTC())
	if err != nil {
		return fmt.Errorf("mark affected weekly reports stale: %w", err)
	}
	return nil
}
