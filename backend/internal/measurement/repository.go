package measurement

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type InitialMeasurement struct {
	ID, UserID               string
	MeasuredAt               time.Time
	WeightKG                 *float64
	ChestCM, WaistCM, HipsCM *float64
	NeckCM, BicepsCM         *float64
}

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) InsertInitial(ctx context.Context, tx pgx.Tx, value InitialMeasurement) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO body_measurements (
			id, user_id, measured_at, weight_kg, chest_cm, waist_cm, hips_cm,
			neck_cm, left_upper_arm_cm, right_upper_arm_cm, source, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, 'import', $3, $3)`,
		value.ID, value.UserID, value.MeasuredAt.UTC(), value.WeightKG, value.ChestCM,
		value.WaistCM, value.HipsCM, value.NeckCM, value.BicepsCM)
	if err != nil {
		return fmt.Errorf("insert initial body measurement: %w", err)
	}
	return nil
}
