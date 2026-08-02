package exercise

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

type Service struct {
	pool       *pgxpool.Pool
	repository *Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(pool *pgxpool.Pool, repository *Repository) *Service {
	return &Service{pool: pool, repository: repository, now: time.Now, newID: id.UUID}
}

func (s *Service) List(ctx context.Context, actorID string, filter ListFilter) (ListResult, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if len([]rune(filter.Query)) > 200 || filter.Limit < 1 || filter.Limit > 100 {
		return ListResult{}, ErrValidation
	}
	if filter.Scope != "" && filter.Scope != "all" && filter.Scope != "system" && filter.Scope != "mine" {
		return ListResult{}, ErrValidation
	}
	if filter.MuscleGroup != "" {
		if _, ok := muscleGroups[filter.MuscleGroup]; !ok {
			return ListResult{}, ErrValidation
		}
	}
	if filter.ExerciseType != "" {
		if _, ok := exerciseTypes[filter.ExerciseType]; !ok {
			return ListResult{}, ErrValidation
		}
	}
	if filter.Equipment != "" {
		if _, ok := equipmentTypes[filter.Equipment]; !ok {
			return ListResult{}, ErrValidation
		}
	}
	return s.repository.List(ctx, actorID, filter)
}

func (s *Service) Get(ctx context.Context, actorID, exerciseID string) (Exercise, error) {
	return s.repository.GetVisible(ctx, actorID, exerciseID)
}

func (s *Service) Create(ctx context.Context, actorID string, input CreateInput) (Exercise, error) {
	normalized, err := normalizeCreate(input)
	if err != nil {
		return Exercise{}, err
	}
	exerciseID, err := s.newID()
	if err != nil {
		return Exercise{}, err
	}
	now := s.now().UTC()
	value := Exercise{
		ID: exerciseID, OwnerUserID: &actorID, Name: normalized.Name,
		Description: normalized.Description, Instructions: normalized.Instructions,
		PrimaryMuscleGroup: normalized.PrimaryMuscleGroup, ExerciseType: normalized.ExerciseType,
		Equipment: normalized.Equipment, MovementPattern: normalized.MovementPattern,
		IsUnilateral: normalized.IsUnilateral, TracksWeight: normalized.TracksWeight,
		TracksRepetitions: normalized.TracksRepetitions, TracksTime: normalized.TracksTime,
		TracksDistance: normalized.TracksDistance, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return s.repository.Insert(ctx, tx, value)
	}); err != nil {
		return Exercise{}, err
	}
	return s.Get(ctx, actorID, exerciseID)
}

func (s *Service) Patch(ctx context.Context, actorID, exerciseID string, expectedVersion int64, input PatchInput) (Exercise, error) {
	if !patchHasFields(input) {
		return Exercise{}, ErrValidation
	}
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		value, err := s.repository.LockVisible(ctx, tx, actorID, exerciseID)
		if err != nil {
			return err
		}
		if value.OwnerUserID == nil {
			return ErrSystemImmutable
		}
		if value.ArchivedAt != nil {
			return ErrArchived
		}
		if value.Version != expectedVersion {
			return ErrVersionConflict
		}
		if err := applyPatch(&value, input); err != nil {
			return err
		}
		return s.repository.Update(ctx, tx, actorID, value, expectedVersion, now)
	})
	if err != nil {
		return Exercise{}, err
	}
	return s.Get(ctx, actorID, exerciseID)
}

func (s *Service) Archive(ctx context.Context, actorID, exerciseID string, expectedVersion int64) error {
	now := s.now().UTC()
	return database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		value, err := s.repository.LockVisible(ctx, tx, actorID, exerciseID)
		if err != nil {
			return err
		}
		if value.OwnerUserID == nil {
			return ErrSystemImmutable
		}
		if value.ArchivedAt != nil {
			return ErrArchived
		}
		if value.Version != expectedVersion {
			return ErrVersionConflict
		}
		return s.repository.Archive(ctx, tx, actorID, exerciseID, expectedVersion, now)
	})
}

func (s *Service) IsUsable(ctx context.Context, tx pgx.Tx, actorID, exerciseID string) (bool, error) {
	return s.repository.IsUsable(ctx, tx, actorID, exerciseID)
}

func applyPatch(value *Exercise, input PatchInput) error {
	if input.Name.Set {
		if input.Name.Value == nil {
			return ErrValidation
		}
		value.Name = strings.TrimSpace(*input.Name.Value)
	}
	if input.Description.Set {
		value.Description = cleanOptional(input.Description.Value)
	}
	if input.Instructions.Set {
		value.Instructions = cleanOptional(input.Instructions.Value)
	}
	if input.PrimaryMuscleGroup.Set {
		value.PrimaryMuscleGroup = cleanOptional(input.PrimaryMuscleGroup.Value)
	}
	if input.ExerciseType.Set {
		if input.ExerciseType.Value == nil {
			return ErrValidation
		}
		value.ExerciseType = strings.TrimSpace(*input.ExerciseType.Value)
	}
	if input.Equipment.Set {
		if input.Equipment.Value == nil {
			return ErrValidation
		}
		value.Equipment = cleanOptional(input.Equipment.Value)
	}
	if input.MovementPattern.Set {
		value.MovementPattern = cleanOptional(input.MovementPattern.Value)
	}
	if input.IsUnilateral.Set {
		if input.IsUnilateral.Value == nil {
			return ErrValidation
		}
		value.IsUnilateral = *input.IsUnilateral.Value
	}
	if input.TracksWeight.Set {
		if input.TracksWeight.Value == nil {
			return ErrValidation
		}
		value.TracksWeight = *input.TracksWeight.Value
	}
	if input.TracksRepetitions.Set {
		if input.TracksRepetitions.Value == nil {
			return ErrValidation
		}
		value.TracksRepetitions = *input.TracksRepetitions.Value
	}
	if input.TracksTime.Set {
		if input.TracksTime.Value == nil {
			return ErrValidation
		}
		value.TracksTime = *input.TracksTime.Value
	}
	if input.TracksDistance.Set {
		if input.TracksDistance.Value == nil {
			return ErrValidation
		}
		value.TracksDistance = *input.TracksDistance.Value
	}
	_, err := normalizeCreate(CreateInput{
		Name: value.Name, Description: value.Description, Instructions: value.Instructions,
		PrimaryMuscleGroup: value.PrimaryMuscleGroup, ExerciseType: value.ExerciseType,
		Equipment: value.Equipment, MovementPattern: value.MovementPattern,
		IsUnilateral: value.IsUnilateral, TracksWeight: value.TracksWeight,
		TracksRepetitions: value.TracksRepetitions, TracksTime: value.TracksTime,
		TracksDistance: value.TracksDistance,
	})
	return err
}
