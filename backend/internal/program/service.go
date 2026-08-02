package program

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

type ExerciseUsability interface {
	IsUsable(context.Context, pgx.Tx, string, string) (bool, error)
}

type Service struct {
	pool       *pgxpool.Pool
	repository *Repository
	exercises  ExerciseUsability
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(pool *pgxpool.Pool, repository *Repository, exercises ExerciseUsability) *Service {
	return &Service{
		pool: pool, repository: repository, exercises: exercises,
		now: time.Now, newID: id.UUID,
	}
}

func (s *Service) List(ctx context.Context, actorID string, filter ListFilter) (ListResult, error) {
	if filter.Limit < 1 || filter.Limit > 100 {
		return ListResult{}, ErrValidation
	}
	switch filter.Status {
	case "", "draft", "active", "inactive", "archived":
	default:
		return ListResult{}, ErrValidation
	}
	if filter.Status == "archived" && !filter.IncludeArchived {
		return ListResult{}, ErrValidation
	}
	return s.repository.List(ctx, actorID, filter)
}

func (s *Service) Get(ctx context.Context, actorID, programID string) (Program, error) {
	return s.repository.Get(ctx, actorID, programID)
}

func (s *Service) Create(ctx context.Context, actorID string, input CreateInput) (Program, error) {
	normalized, err := normalizeCreate(input)
	if err != nil {
		return Program{}, err
	}
	programID, err := s.newID()
	if err != nil {
		return Program{}, err
	}
	now := s.now().UTC()
	root := Program{
		ID: programID, Name: normalized.Name, Description: normalized.Description,
		Goal: normalized.Goal, Status: "draft", Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		days, err := s.buildTree(ctx, tx, actorID, normalized.Days, now)
		if err != nil {
			return err
		}
		if err := s.repository.InsertRoot(ctx, tx, actorID, root); err != nil {
			return err
		}
		return s.repository.InsertTree(ctx, tx, actorID, programID, days, now)
	})
	if err != nil {
		return Program{}, err
	}
	return s.Get(ctx, actorID, programID)
}

func (s *Service) Patch(ctx context.Context, actorID, programID string, expectedVersion int64, input PatchInput) (Program, error) {
	if !patchHasFields(input) {
		return Program{}, ErrValidation
	}
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		value, err := s.repository.Lock(ctx, tx, actorID, programID)
		if err != nil {
			return err
		}
		if value.Status == "archived" {
			return ErrArchived
		}
		if value.Version != expectedVersion {
			return ErrVersionConflict
		}
		updated := inputFromProgram(value)
		if input.Name.Set {
			if input.Name.Value == nil {
				return ErrValidation
			}
			updated.Name = *input.Name.Value
		}
		if input.Description.Set {
			updated.Description = input.Description.Value
		}
		if input.Goal.Set {
			updated.Goal = input.Goal.Value
		}
		if input.Days.Set {
			if input.Days.Value == nil {
				return ErrValidation
			}
			updated.Days = *input.Days.Value
		}
		normalized, err := normalizeCreate(updated)
		if err != nil {
			return err
		}
		if value.Status == "active" && !validActivationInput(normalized.Days) {
			return ErrNotActivatable
		}
		if input.Days.Set {
			days, err := s.buildTree(ctx, tx, actorID, normalized.Days, now)
			if err != nil {
				return err
			}
			if err := s.repository.ArchiveTree(ctx, tx, actorID, programID, now); err != nil {
				return err
			}
			if err := s.repository.InsertTree(ctx, tx, actorID, programID, days, now); err != nil {
				return err
			}
		}
		value.Name, value.Description, value.Goal = normalized.Name, normalized.Description, normalized.Goal
		return s.repository.UpdateRoot(ctx, tx, actorID, value, expectedVersion, now)
	})
	if err != nil {
		return Program{}, err
	}
	return s.Get(ctx, actorID, programID)
}

func (s *Service) Archive(ctx context.Context, actorID, programID string, expectedVersion int64) error {
	now := s.now().UTC()
	return database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		value, err := s.repository.Lock(ctx, tx, actorID, programID)
		if err != nil {
			return err
		}
		if value.Status == "archived" {
			return ErrArchived
		}
		if value.Version != expectedVersion {
			return ErrVersionConflict
		}
		return s.repository.Archive(ctx, tx, actorID, programID, expectedVersion, now)
	})
}

func (s *Service) Duplicate(ctx context.Context, actorID, programID string, requestedName *string) (Program, error) {
	now := s.now().UTC()
	newProgramID, err := s.newID()
	if err != nil {
		return Program{}, err
	}
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		source, err := s.repository.Lock(ctx, tx, actorID, programID)
		if err != nil {
			return err
		}
		if source.Status == "archived" {
			return ErrArchived
		}
		input := inputFromProgram(source)
		if requestedName != nil {
			input.Name = *requestedName
		} else {
			input.Name = copyName(source.Name)
		}
		normalized, err := normalizeCreate(input)
		if err != nil {
			return err
		}
		days, err := s.buildTree(ctx, tx, actorID, normalized.Days, now)
		if err != nil {
			return err
		}
		root := Program{
			ID: newProgramID, Name: normalized.Name, Description: normalized.Description,
			Goal: normalized.Goal, Status: "draft", Version: 1,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := s.repository.InsertRoot(ctx, tx, actorID, root); err != nil {
			return err
		}
		return s.repository.InsertTree(ctx, tx, actorID, newProgramID, days, now)
	})
	if err != nil {
		return Program{}, err
	}
	return s.Get(ctx, actorID, newProgramID)
}

func (s *Service) Activate(ctx context.Context, actorID, programID string, expectedVersion int64) (Program, error) {
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		roots, err := s.repository.LockRootsForActivation(ctx, tx, actorID)
		if err != nil {
			return err
		}
		var target *Program
		for index := range roots {
			if roots[index].ID == programID {
				target = &roots[index]
				break
			}
		}
		if target == nil {
			return ErrNotFound
		}
		if target.Status == "archived" {
			return ErrArchived
		}
		if target.Version != expectedVersion {
			return ErrVersionConflict
		}
		if target.Status == "active" {
			return nil
		}
		aggregate, err := s.repository.Lock(ctx, tx, actorID, programID)
		if err != nil {
			return err
		}
		if !validActivation(aggregate) {
			return ErrNotActivatable
		}
		for _, day := range aggregate.Days {
			for _, item := range day.Exercises {
				usable, err := s.exercises.IsUsable(ctx, tx, actorID, item.ExerciseID)
				if err != nil {
					return err
				}
				if !usable {
					return ErrExerciseUnavailable
				}
			}
		}
		if err := s.repository.DeactivateCurrent(ctx, tx, actorID, programID, now); err != nil {
			return err
		}
		return s.repository.ActivateTarget(ctx, tx, actorID, programID, expectedVersion, now)
	})
	if err != nil {
		return Program{}, err
	}
	return s.Get(ctx, actorID, programID)
}

func (s *Service) buildTree(ctx context.Context, tx pgx.Tx, actorID string, input []DayInput, now time.Time) ([]ProgramDay, error) {
	days := make([]ProgramDay, 0, len(input))
	for _, sourceDay := range input {
		dayID, err := s.newID()
		if err != nil {
			return nil, err
		}
		day := ProgramDay{
			ID: dayID, Position: sourceDay.Position, Name: sourceDay.Name,
			Notes: sourceDay.Notes, Exercises: []ProgramDayExercise{},
			CreatedAt: now, UpdatedAt: now,
		}
		for _, sourceItem := range sourceDay.Exercises {
			usable, err := s.exercises.IsUsable(ctx, tx, actorID, sourceItem.ExerciseID)
			if err != nil {
				return nil, err
			}
			if !usable {
				return nil, ErrExerciseUnavailable
			}
			itemID, err := s.newID()
			if err != nil {
				return nil, err
			}
			day.Exercises = append(day.Exercises, ProgramDayExercise{
				ID: itemID, ExerciseID: sourceItem.ExerciseID, Position: sourceItem.Position,
				WorkingSets: sourceItem.WorkingSets, TargetRepsMin: sourceItem.TargetRepsMin,
				TargetRepsMax: sourceItem.TargetRepsMax, TargetRIR: sourceItem.TargetRIR,
				RestSeconds: sourceItem.RestSeconds, Notes: sourceItem.Notes,
				CreatedAt: now, UpdatedAt: now,
			})
		}
		days = append(days, day)
	}
	return days, nil
}

func inputFromProgram(value Program) CreateInput {
	input := CreateInput{
		Name: value.Name, Description: value.Description, Goal: value.Goal,
		Days: make([]DayInput, 0, len(value.Days)),
	}
	for _, sourceDay := range value.Days {
		day := DayInput{
			Position: sourceDay.Position, Name: sourceDay.Name, Notes: sourceDay.Notes,
			Exercises: make([]DayExerciseInput, 0, len(sourceDay.Exercises)),
		}
		for _, sourceItem := range sourceDay.Exercises {
			day.Exercises = append(day.Exercises, DayExerciseInput{
				ExerciseID: sourceItem.ExerciseID, Position: sourceItem.Position,
				WorkingSets: sourceItem.WorkingSets, TargetRepsMin: sourceItem.TargetRepsMin,
				TargetRepsMax: sourceItem.TargetRepsMax, TargetRIR: sourceItem.TargetRIR,
				RestSeconds: sourceItem.RestSeconds, Notes: sourceItem.Notes,
			})
		}
		input.Days = append(input.Days, day)
	}
	return input
}

func copyName(name string) string {
	const suffix = " (копия)"
	maximum := 200 - utf8.RuneCountInString(suffix)
	runes := []rune(name)
	if len(runes) > maximum {
		runes = runes[:maximum]
	}
	return string(runes) + suffix
}
