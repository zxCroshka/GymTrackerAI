package measurement

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/calendartime"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

type TimezoneReader interface {
	Timezone(context.Context, string) (string, error)
}

type ReportInvalidator interface {
	LockUser(context.Context, pgx.Tx, string) error
	MarkPeriodsStale(context.Context, pgx.Tx, string, []time.Time, time.Time) error
}

type Service struct {
	pool       *pgxpool.Pool
	repository *Repository
	timezones  TimezoneReader
	reports    ReportInvalidator
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(pool *pgxpool.Pool, repository *Repository, timezones TimezoneReader, reports ReportInvalidator) *Service {
	return &Service{pool: pool, repository: repository, timezones: timezones, reports: reports, now: time.Now, newID: id.UUID}
}

func (s *Service) CreateBody(ctx context.Context, actorID string, input BodyCreateInput) (BodyMeasurement, error) {
	now := s.now().UTC()
	value := BodyMeasurement{MeasuredAt: input.MeasuredAt.UTC(), WeightKG: input.WeightKG,
		ChestCM: input.ChestCM, WaistCM: input.WaistCM, HipsCM: input.HipsCM, NeckCM: input.NeckCM,
		LeftUpperArmCM: input.LeftUpperArmCM, RightUpperArmCM: input.RightUpperArmCM,
		LeftThighCM: input.LeftThighCM, RightThighCM: input.RightThighCM,
		BodyFatPercent: input.BodyFatPercent, Notes: cleanNote(input.Notes), Source: "manual",
		Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := validateBody(value); err != nil {
		return BodyMeasurement{}, err
	}
	var err error
	value.ID, err = s.newID()
	if err != nil {
		return BodyMeasurement{}, err
	}
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := s.reports.LockUser(ctx, tx, actorID); err != nil {
			return err
		}
		if err := s.repository.InsertBody(ctx, tx, actorID, value); err != nil {
			return err
		}
		return s.reports.MarkPeriodsStale(ctx, tx, actorID, []time.Time{value.MeasuredAt}, now)
	})
	if err != nil {
		return BodyMeasurement{}, err
	}
	return value, nil
}

func (s *Service) ListBody(ctx context.Context, actorID string, filter ListFilter) (BodyListResult, error) {
	if err := validateFilter(filter); err != nil {
		return BodyListResult{}, err
	}
	return s.repository.ListBody(ctx, actorID, filter)
}

func (s *Service) PatchBody(ctx context.Context, actorID, measurementID string, expectedVersion int64, input BodyPatchInput) (BodyMeasurement, error) {
	if !id.ValidUUID(measurementID) || !bodyPatchHasFields(input) {
		return BodyMeasurement{}, ErrValidation
	}
	now := s.now().UTC()
	var value BodyMeasurement
	var oldAt time.Time
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := s.reports.LockUser(ctx, tx, actorID); err != nil {
			return err
		}
		current, err := s.repository.LockBody(ctx, tx, actorID, measurementID)
		if err != nil {
			return err
		}
		if current.Version != expectedVersion {
			return ErrVersionConflict
		}
		oldAt = current.MeasuredAt
		if input.MeasuredAt.Set {
			if input.MeasuredAt.Value == nil {
				return ErrValidation
			}
			current.MeasuredAt = input.MeasuredAt.Value.UTC()
		}
		applyOptional(&current.WeightKG, input.WeightKG)
		applyOptional(&current.ChestCM, input.ChestCM)
		applyOptional(&current.WaistCM, input.WaistCM)
		applyOptional(&current.HipsCM, input.HipsCM)
		applyOptional(&current.NeckCM, input.NeckCM)
		applyOptional(&current.LeftUpperArmCM, input.LeftUpperArmCM)
		applyOptional(&current.RightUpperArmCM, input.RightUpperArmCM)
		applyOptional(&current.LeftThighCM, input.LeftThighCM)
		applyOptional(&current.RightThighCM, input.RightThighCM)
		applyOptional(&current.BodyFatPercent, input.BodyFatPercent)
		if input.Notes.Set {
			current.Notes = cleanNote(input.Notes.Value)
		}
		if err := validateBody(current); err != nil {
			return err
		}
		if err := s.repository.UpdateBody(ctx, tx, actorID, current, expectedVersion, now); err != nil {
			return err
		}
		if err := s.reports.MarkPeriodsStale(ctx, tx, actorID, []time.Time{oldAt, current.MeasuredAt}, now); err != nil {
			return err
		}
		current.Version++
		current.UpdatedAt = now
		value = current
		return nil
	})
	return value, err
}

func (s *Service) DeleteBody(ctx context.Context, actorID, measurementID string, expectedVersion int64) error {
	if !id.ValidUUID(measurementID) {
		return ErrValidation
	}
	now := s.now().UTC()
	return database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := s.reports.LockUser(ctx, tx, actorID); err != nil {
			return err
		}
		current, err := s.repository.LockBody(ctx, tx, actorID, measurementID)
		if err != nil {
			return err
		}
		if current.Version != expectedVersion {
			return ErrVersionConflict
		}
		if err := s.repository.DeleteBody(ctx, tx, actorID, measurementID, expectedVersion); err != nil {
			return err
		}
		return s.reports.MarkPeriodsStale(ctx, tx, actorID, []time.Time{current.MeasuredAt}, now)
	})
}

func (s *Service) CreateWellness(ctx context.Context, actorID string, input WellnessCreateInput) (WellnessEntry, error) {
	if err := validateWellness(input); err != nil {
		return WellnessEntry{}, err
	}
	timezone, err := s.timezones.Timezone(ctx, actorID)
	if err != nil {
		return WellnessEntry{}, err
	}
	dayStart, err := calendartime.DayStart(input.ObservedAt, timezone)
	if err != nil {
		return WellnessEntry{}, ErrValidation
	}
	now := s.now().UTC()
	entryID, err := s.newID()
	if err != nil {
		return WellnessEntry{}, err
	}
	value := WellnessEntry{ID: entryID, ObservedAt: input.ObservedAt.UTC(), DayStartAt: dayStart,
		TimezoneAtEntry: timezone, SleepMinutes: input.SleepMinutes, SleepQuality: input.SleepQuality,
		Energy: input.Energy, Steps: input.Steps, CaloriesKcal: input.CaloriesKcal,
		ProteinG: input.ProteinG, FatG: input.FatG, CarbsG: input.CarbsG, Notes: cleanNote(input.Notes),
		Version: 1, CreatedAt: now, UpdatedAt: now}
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := s.reports.LockUser(ctx, tx, actorID); err != nil {
			return err
		}
		if err := s.repository.InsertWellness(ctx, tx, actorID, value); err != nil {
			return err
		}
		return s.reports.MarkPeriodsStale(ctx, tx, actorID, []time.Time{dayStart}, now)
	})
	return value, err
}

func (s *Service) ListWellness(ctx context.Context, actorID string, filter ListFilter) (WellnessListResult, error) {
	if err := validateFilter(filter); err != nil {
		return WellnessListResult{}, err
	}
	return s.repository.ListWellness(ctx, actorID, filter)
}

func (s *Service) WeightSummary(ctx context.Context, actorID string, now time.Time) (WeightSummary, error) {
	return s.repository.WeightSummary(ctx, actorID, now.UTC())
}

func (s *Service) WeightPoints(ctx context.Context, actorID string, from, to time.Time) ([]WeightPoint, error) {
	if !from.Before(to) || to.Sub(from) > 2*365*24*time.Hour {
		return nil, ErrValidation
	}
	return s.repository.WeightPoints(ctx, actorID, from.UTC(), to.UTC())
}

func (s *Service) ReportSnapshot(ctx context.Context, tx pgx.Tx, actorID string, from, to time.Time) (WeightTrend, WellnessSummary, error) {
	body, err := s.repository.WeightTrend(ctx, tx, actorID, from, to)
	if err != nil {
		return WeightTrend{}, WellnessSummary{}, err
	}
	wellness, err := s.repository.WellnessSummary(ctx, tx, actorID, from, to)
	return body, wellness, err
}

func applyOptional[T any](target **T, input Optional[T]) {
	if input.Set {
		*target = input.Value
	}
}
