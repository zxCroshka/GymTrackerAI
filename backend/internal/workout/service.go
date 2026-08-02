package workout

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/exercise"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/program"
)

type ProgramSnapshotter interface {
	SnapshotActiveDay(context.Context, pgx.Tx, string, string) (program.WorkoutDaySnapshot, error)
}

type ExerciseSnapshotter interface {
	SnapshotForWorkout(context.Context, pgx.Tx, string, string, bool) (exercise.WorkoutSnapshot, error)
}

type Service struct {
	pool       *pgxpool.Pool
	repository *Repository
	programs   ProgramSnapshotter
	exercises  ExerciseSnapshotter
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(pool *pgxpool.Pool, repository *Repository, programs ProgramSnapshotter, exercises ExerciseSnapshotter) *Service {
	return &Service{
		pool: pool, repository: repository, programs: programs, exercises: exercises,
		now: time.Now, newID: id.UUID,
	}
}

func (s *Service) Create(ctx context.Context, actorID string, input CreateInput) (Workout, error) {
	now := s.now().UTC()
	normalized, err := normalizeCreate(input, now)
	if err != nil {
		return Workout{}, err
	}
	if normalized.ProgramDayID != nil && !id.ValidUUID(*normalized.ProgramDayID) {
		return Workout{}, ErrValidation
	}
	workoutID, err := s.newID()
	if err != nil {
		return Workout{}, err
	}
	value := Workout{
		ID: workoutID, Status: normalized.Status, ScheduledAt: normalized.ScheduledAt,
		StartedAt: normalized.StartedAt, Difficulty: normalized.Difficulty, Energy: normalized.Energy,
		Mood: normalized.Mood, Comment: normalized.Comment, HasPain: normalized.HasPain,
		Discomfort: normalized.Discomfort, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if normalized.ProgramDayID == nil {
			value.Name = *normalized.Name
		} else {
			snapshot, err := s.programs.SnapshotActiveDay(ctx, tx, actorID, *normalized.ProgramDayID)
			if errors.Is(err, program.ErrNotFound) {
				return ErrProgramUnavailable
			}
			if err != nil {
				return err
			}
			value.Name = snapshot.DayName
			value.SourceProgramID = &snapshot.ProgramID
			value.SourceProgramDayID = &snapshot.DayID
			value.SourceProgramVersion = &snapshot.ProgramVersion
			if err := s.repository.InsertRoot(ctx, tx, actorID, value); err != nil {
				return err
			}
			return s.insertProgramTree(ctx, tx, actorID, value.ID, snapshot, now)
		}
		return s.repository.InsertRoot(ctx, tx, actorID, value)
	})
	if err != nil {
		return Workout{}, err
	}
	return s.Get(ctx, actorID, workoutID)
}

func (s *Service) insertProgramTree(ctx context.Context, tx pgx.Tx, actorID, workoutID string, snapshot program.WorkoutDaySnapshot, now time.Time) error {
	for _, prescription := range snapshot.Items {
		metadata, err := s.exercises.SnapshotForWorkout(ctx, tx, actorID, prescription.ExerciseID, true)
		if errors.Is(err, exercise.ErrNotFound) {
			return ErrExerciseUnavailable
		}
		if err != nil {
			return err
		}
		itemID, err := s.newID()
		if err != nil {
			return err
		}
		item := WorkoutExercise{
			ID: itemID, WorkoutID: workoutID, ExerciseID: metadata.ID,
			SourceProgramDayExerciseID: &prescription.SourceItemID, Position: prescription.Position,
			ExerciseNameSnapshot: metadata.Name, Comment: prescription.Notes,
			RestSeconds: prescription.RestSeconds, TracksWeight: metadata.TracksWeight,
			TracksRepetitions: metadata.TracksRepetitions, TracksTime: metadata.TracksTime,
			TracksDistance: metadata.TracksDistance,
		}
		if err := s.repository.InsertExercise(ctx, tx, actorID, item, now); err != nil {
			return err
		}
		for position := int16(1); position <= prescription.WorkingSets; position++ {
			setID, err := s.newID()
			if err != nil {
				return err
			}
			set := WorkoutSet{
				ID: setID, WorkoutExerciseID: itemID, SetNumber: position,
				TargetRepsMin: prescription.TargetRepsMin, TargetRepsMax: prescription.TargetRepsMax,
				TargetRIR: prescription.TargetRIR,
			}
			if err := s.repository.InsertPlannedSet(ctx, tx, actorID, set, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context, actorID string, filter ListFilter) (ListResult, error) {
	if err := validateListFilter(filter); err != nil {
		return ListResult{}, err
	}
	return s.repository.List(ctx, actorID, filter)
}

func (s *Service) Active(ctx context.Context, actorID string) (*Workout, error) {
	return s.repository.Active(ctx, actorID)
}

func (s *Service) Get(ctx context.Context, actorID, workoutID string) (Workout, error) {
	return s.repository.Get(ctx, actorID, workoutID)
}

func (s *Service) Patch(ctx context.Context, actorID, workoutID string, expectedVersion int64, input PatchInput) (Workout, error) {
	if err := validatePatch(input); err != nil {
		return Workout{}, err
	}
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		value, err := s.repository.Lock(ctx, tx, actorID, workoutID)
		if err != nil {
			return err
		}
		if value.Version != expectedVersion {
			return ErrVersionConflict
		}
		if value.Status == "cancelled" {
			return ErrInvalidState
		}
		if err := applyWorkoutPatch(&value, input, now); err != nil {
			return err
		}
		minimum, maximum, err := s.repository.PerformedRange(ctx, tx, actorID, workoutID)
		if err != nil {
			return err
		}
		if err := validateWorkoutTimes(value, minimum, maximum); err != nil {
			return err
		}
		return s.repository.UpdateRoot(ctx, tx, actorID, value, expectedVersion, now)
	})
	if err != nil {
		return Workout{}, err
	}
	return s.Get(ctx, actorID, workoutID)
}

func applyWorkoutPatch(value *Workout, input PatchInput, now time.Time) error {
	if input.Name.Set {
		name, err := requiredText(*input.Name.Value, 200)
		if err != nil {
			return err
		}
		value.Name = name
	}
	if input.ScheduledAt.Set {
		value.ScheduledAt = utcTime(input.ScheduledAt.Value)
	}
	if input.StartedAt.Set {
		value.StartedAt = utcTime(input.StartedAt.Value)
	}
	if input.CompletedAt.Set {
		if value.Status != "completed" || input.CompletedAt.Value == nil {
			return ErrInvalidState
		}
		value.CompletedAt = utcTime(input.CompletedAt.Value)
	}
	applyOptional(&value.Difficulty, input.Difficulty)
	applyOptional(&value.Energy, input.Energy)
	applyOptional(&value.Mood, input.Mood)
	if err := applyOptionalText(&value.Comment, input.Comment, 4000); err != nil {
		return err
	}
	applyOptional(&value.HasPain, input.HasPain)
	if err := applyOptionalText(&value.Discomfort, input.Discomfort, 4000); err != nil {
		return err
	}
	if !input.Status.Set {
		return nil
	}
	target := *input.Status.Value
	switch {
	case value.Status == "planned" && target == "in_progress":
		value.Status = target
		if value.StartedAt == nil {
			started := now.UTC()
			value.StartedAt = &started
		}
	case (value.Status == "planned" || value.Status == "in_progress") && target == "cancelled":
		value.Status = target
		cancelled := now.UTC()
		value.CancelledAt = &cancelled
	case value.Status == "in_progress" && target == "in_progress":
		return nil
	default:
		return ErrInvalidState
	}
	return nil
}

func (s *Service) Complete(ctx context.Context, actorID, workoutID string, expectedVersion int64, input CompleteInput) (Workout, error) {
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		value, err := s.repository.Lock(ctx, tx, actorID, workoutID)
		if err != nil {
			return err
		}
		if value.Status == "completed" {
			return nil
		}
		if err := validateComplete(input); err != nil {
			return err
		}
		if value.Version != expectedVersion {
			return ErrVersionConflict
		}
		if value.Status != "in_progress" || value.StartedAt == nil {
			return ErrInvalidState
		}
		completedAt := now
		if input.CompletedAt.Set {
			completedAt = input.CompletedAt.Value.UTC()
		}
		value.Status = "completed"
		value.CompletedAt = &completedAt
		applyOptional(&value.Difficulty, input.Difficulty)
		applyOptional(&value.Energy, input.Energy)
		applyOptional(&value.Mood, input.Mood)
		if err := applyOptionalText(&value.Comment, input.Comment, 4000); err != nil {
			return err
		}
		applyOptional(&value.HasPain, input.HasPain)
		if err := applyOptionalText(&value.Discomfort, input.Discomfort, 4000); err != nil {
			return err
		}
		minimum, maximum, err := s.repository.PerformedRange(ctx, tx, actorID, workoutID)
		if err != nil {
			return err
		}
		if err := validateWorkoutTimes(value, minimum, maximum); err != nil {
			return err
		}
		if err := s.repository.MarkRemainingSetsSkipped(ctx, tx, actorID, workoutID, now); err != nil {
			return err
		}
		return s.repository.UpdateRoot(ctx, tx, actorID, value, expectedVersion, now)
	})
	if err != nil {
		return Workout{}, err
	}
	return s.Get(ctx, actorID, workoutID)
}

func (s *Service) Delete(ctx context.Context, actorID, workoutID string, expectedVersion int64) error {
	return database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		value, err := s.repository.Lock(ctx, tx, actorID, workoutID)
		if err != nil {
			return err
		}
		if value.Version != expectedVersion {
			return ErrVersionConflict
		}
		return s.repository.Delete(ctx, tx, actorID, workoutID)
	})
}

func (s *Service) AddExercise(ctx context.Context, actorID, workoutID string, expectedVersion int64, input ExerciseCreateInput) (WorkoutExercise, int64, error) {
	if err := validateExerciseCreate(input); err != nil || !id.ValidUUID(input.ExerciseID) {
		return WorkoutExercise{}, 0, ErrValidation
	}
	now := s.now().UTC()
	var result WorkoutExercise
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		root, err := s.repository.Lock(ctx, tx, actorID, workoutID)
		if err != nil {
			return err
		}
		if root.Version != expectedVersion {
			return ErrVersionConflict
		}
		if root.Status == "cancelled" {
			return ErrInvalidState
		}
		count, err := s.repository.ExerciseCount(ctx, tx, actorID, workoutID)
		if err != nil {
			return err
		}
		if count >= 50 {
			return ErrValidation
		}
		position := int16(count + 1)
		if input.Position != nil {
			position = *input.Position
			if position > int16(count+1) {
				return ErrValidation
			}
		}
		metadata, err := s.exercises.SnapshotForWorkout(ctx, tx, actorID, input.ExerciseID, false)
		if errors.Is(err, exercise.ErrNotFound) {
			return ErrExerciseUnavailable
		}
		if err != nil {
			return err
		}
		itemID, err := s.newID()
		if err != nil {
			return err
		}
		comment, err := optionalText(input.Comment, 4000)
		if err != nil {
			return err
		}
		result = WorkoutExercise{
			ID: itemID, WorkoutID: workoutID, ExerciseID: metadata.ID, Position: position,
			ExerciseNameSnapshot: metadata.Name, Comment: comment, TracksWeight: metadata.TracksWeight,
			TracksRepetitions: metadata.TracksRepetitions, TracksTime: metadata.TracksTime,
			TracksDistance: metadata.TracksDistance,
		}
		if err := s.repository.ShiftExercisesForInsert(ctx, tx, actorID, workoutID, position); err != nil {
			return err
		}
		if err := s.repository.InsertExercise(ctx, tx, actorID, result, now); err != nil {
			return err
		}
		return s.repository.Touch(ctx, tx, actorID, workoutID, expectedVersion, now)
	})
	if err != nil {
		return WorkoutExercise{}, 0, err
	}
	result.CreatedAt, result.UpdatedAt = now, now
	result.Sets = []WorkoutSet{}
	return result, expectedVersion + 1, nil
}

func (s *Service) PatchExercise(ctx context.Context, actorID, itemID, expectedWorkoutID string, expectedVersion int64, input ExercisePatchInput) (WorkoutExercise, int64, error) {
	if err := validateExercisePatch(input); err != nil {
		return WorkoutExercise{}, 0, err
	}
	now := s.now().UTC()
	var result WorkoutExercise
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		root, item, err := s.repository.LockByExercise(ctx, tx, actorID, itemID)
		if err != nil {
			return err
		}
		if root.ID != expectedWorkoutID {
			return ErrVersionConflict
		}
		if root.Version != expectedVersion {
			return ErrVersionConflict
		}
		if root.Status == "cancelled" {
			return ErrInvalidState
		}
		if input.Position.Set {
			count, err := s.repository.ExerciseCount(ctx, tx, actorID, root.ID)
			if err != nil {
				return err
			}
			if int(*input.Position.Value) > count {
				return ErrValidation
			}
			if err := s.repository.MoveExercise(ctx, tx, actorID, root.ID, item.ID, item.Position, *input.Position.Value); err != nil {
				return err
			}
			item.Position = *input.Position.Value
		}
		if err := applyOptionalText(&item.Comment, input.Comment, 4000); err != nil {
			return err
		}
		if err := s.repository.UpdateExercise(ctx, tx, actorID, item, now); err != nil {
			return err
		}
		if err := s.repository.Touch(ctx, tx, actorID, root.ID, expectedVersion, now); err != nil {
			return err
		}
		result = item
		return nil
	})
	if err != nil {
		return WorkoutExercise{}, 0, err
	}
	result.UpdatedAt = now
	return result, expectedVersion + 1, nil
}

func (s *Service) DeleteExercise(ctx context.Context, actorID, itemID, expectedWorkoutID string, expectedVersion int64) (int64, error) {
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		root, item, err := s.repository.LockByExercise(ctx, tx, actorID, itemID)
		if err != nil {
			return err
		}
		if root.ID != expectedWorkoutID {
			return ErrVersionConflict
		}
		if root.Version != expectedVersion {
			return ErrVersionConflict
		}
		if root.Status == "cancelled" {
			return ErrInvalidState
		}
		if err := s.repository.DeleteExercise(ctx, tx, actorID, item); err != nil {
			return err
		}
		return s.repository.Touch(ctx, tx, actorID, root.ID, expectedVersion, now)
	})
	return expectedVersion + 1, err
}

func (s *Service) AddSet(ctx context.Context, actorID, itemID, expectedWorkoutID string, expectedVersion int64, input SetCreateInput) (WorkoutSet, int64, error) {
	now := s.now().UTC()
	var result WorkoutSet
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		root, item, err := s.repository.LockByExercise(ctx, tx, actorID, itemID)
		if err != nil {
			return err
		}
		if root.ID != expectedWorkoutID {
			return ErrVersionConflict
		}
		if root.Version != expectedVersion {
			return ErrVersionConflict
		}
		if root.Status != "in_progress" && root.Status != "completed" {
			return ErrInvalidState
		}
		normalized, err := normalizeSetCreate(input, item, now)
		if err != nil {
			return err
		}
		if err := validatePerformedAt(root, normalized.PerformedAt); err != nil {
			return err
		}
		count, err := s.repository.SetCount(ctx, tx, actorID, itemID)
		if err != nil {
			return err
		}
		if count >= 100 {
			return ErrValidation
		}
		position := int16(count + 1)
		if normalized.SetNumber != nil {
			position = *normalized.SetNumber
			if position > int16(count+1) {
				return ErrValidation
			}
		}
		setID, err := s.newID()
		if err != nil {
			return err
		}
		result = WorkoutSet{
			ID: setID, WorkoutExerciseID: itemID, SetNumber: position, Status: "completed",
			SetType:  setType(normalized.Warmup, normalized.Failure),
			WeightKG: normalized.WeightKG, Repetitions: normalized.Repetitions, RIR: normalized.RIR,
			Warmup: normalized.Warmup, Failure: normalized.Failure,
			DurationSeconds: normalized.DurationSeconds, DistanceMeters: normalized.DistanceMeters,
			Note: normalized.Note, PerformedAt: normalized.PerformedAt,
		}
		if err := s.repository.ShiftSetsForInsert(ctx, tx, actorID, itemID, position); err != nil {
			return err
		}
		if err := s.repository.InsertCompletedSet(ctx, tx, actorID, result, now); err != nil {
			return err
		}
		return s.repository.Touch(ctx, tx, actorID, root.ID, expectedVersion, now)
	})
	if err != nil {
		return WorkoutSet{}, 0, err
	}
	result.CreatedAt, result.UpdatedAt = now, now
	result.VolumeKG = Volume(result.WeightKG, result.Repetitions, result.Warmup)
	result.Estimated1RMKG = Estimated1RM(result.WeightKG, result.Repetitions, result.Warmup)
	return result, expectedVersion + 1, nil
}

func (s *Service) PatchSet(ctx context.Context, actorID, setID, expectedWorkoutID string, expectedVersion int64, input SetPatchInput) (WorkoutSet, int64, error) {
	if err := validateSetPatch(input); err != nil {
		return WorkoutSet{}, 0, err
	}
	now := s.now().UTC()
	var result WorkoutSet
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		root, item, set, err := s.repository.LockBySet(ctx, tx, actorID, setID)
		if err != nil {
			return err
		}
		if root.ID != expectedWorkoutID {
			return ErrVersionConflict
		}
		if root.Version != expectedVersion {
			return ErrVersionConflict
		}
		if root.Status == "cancelled" {
			return ErrInvalidState
		}
		if err := applySetPatch(&set, input, now); err != nil {
			return err
		}
		if err := validateActualValues(set.WeightKG, set.Repetitions, set.RIR, set.DurationSeconds, set.DistanceMeters); err != nil {
			return err
		}
		if set.Warmup && set.Failure {
			return ErrValidation
		}
		if err := validateCapabilities(item, set.WeightKG, set.Repetitions, set.DurationSeconds, set.DistanceMeters); err != nil {
			return err
		}
		hasActual := set.WeightKG != nil || set.Repetitions != nil || set.DurationSeconds != nil || set.DistanceMeters != nil
		if hasActual {
			if root.Status == "planned" {
				return ErrInvalidState
			}
			set.Status = "completed"
			if set.PerformedAt == nil {
				performed := now
				set.PerformedAt = &performed
			}
			if err := validatePerformedAt(root, set.PerformedAt); err != nil {
				return err
			}
		} else {
			if set.Status == "completed" || set.RIR != nil || set.PerformedAt != nil {
				return ErrValidation
			}
		}
		if input.SetNumber.Set {
			count, err := s.repository.SetCount(ctx, tx, actorID, item.ID)
			if err != nil {
				return err
			}
			if int(*input.SetNumber.Value) > count {
				return ErrValidation
			}
			if err := s.repository.MoveSet(ctx, tx, actorID, item.ID, set.ID, set.SetNumber, *input.SetNumber.Value); err != nil {
				return err
			}
			set.SetNumber = *input.SetNumber.Value
		}
		if err := s.repository.UpdateSet(ctx, tx, actorID, set, now); err != nil {
			return err
		}
		if err := s.repository.Touch(ctx, tx, actorID, root.ID, expectedVersion, now); err != nil {
			return err
		}
		result = set
		return nil
	})
	if err != nil {
		return WorkoutSet{}, 0, err
	}
	result.UpdatedAt = now
	result.VolumeKG = Volume(result.WeightKG, result.Repetitions, result.Warmup)
	result.Estimated1RMKG = Estimated1RM(result.WeightKG, result.Repetitions, result.Warmup)
	return result, expectedVersion + 1, nil
}

func applySetPatch(set *WorkoutSet, input SetPatchInput, now time.Time) error {
	applyOptional(&set.WeightKG, input.WeightKG)
	applyOptional(&set.Repetitions, input.Repetitions)
	applyOptional(&set.RIR, input.RIR)
	if input.Warmup.Set {
		if input.Warmup.Value == nil {
			return ErrValidation
		}
		set.Warmup = *input.Warmup.Value
	}
	if input.Failure.Set {
		if input.Failure.Value == nil {
			return ErrValidation
		}
		set.Failure = *input.Failure.Value
	}
	if input.Warmup.Set || input.Failure.Set {
		set.SetType = setType(set.Warmup, set.Failure)
	}
	applyOptional(&set.DurationSeconds, input.DurationSeconds)
	applyOptional(&set.DistanceMeters, input.DistanceMeters)
	if err := applyOptionalText(&set.Note, input.Note, 4000); err != nil {
		return err
	}
	if input.PerformedAt.Set {
		set.PerformedAt = utcTime(input.PerformedAt.Value)
	}
	_ = now
	return nil
}

func (s *Service) DeleteSet(ctx context.Context, actorID, setID, expectedWorkoutID string, expectedVersion int64) (int64, error) {
	now := s.now().UTC()
	err := database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		root, _, set, err := s.repository.LockBySet(ctx, tx, actorID, setID)
		if err != nil {
			return err
		}
		if root.ID != expectedWorkoutID {
			return ErrVersionConflict
		}
		if root.Version != expectedVersion {
			return ErrVersionConflict
		}
		if root.Status == "cancelled" {
			return ErrInvalidState
		}
		if err := s.repository.DeleteSet(ctx, tx, actorID, set); err != nil {
			return err
		}
		return s.repository.Touch(ctx, tx, actorID, root.ID, expectedVersion, now)
	})
	return expectedVersion + 1, err
}

func (s *Service) PreviousResult(ctx context.Context, actorID, itemID string) (*PreviousResult, error) {
	return s.repository.PreviousResult(ctx, actorID, itemID)
}

func validateWorkoutTimes(value Workout, minimum, maximum *time.Time) error {
	if value.Status == "planned" && value.StartedAt != nil || value.Status == "in_progress" && value.StartedAt == nil ||
		value.Status == "completed" && (value.StartedAt == nil || value.CompletedAt == nil) ||
		value.Status == "cancelled" && value.CancelledAt == nil {
		return ErrValidation
	}
	if value.StartedAt != nil && minimum != nil && minimum.Before(*value.StartedAt) {
		return ErrValidation
	}
	if value.CompletedAt != nil {
		if value.StartedAt == nil || value.CompletedAt.Before(*value.StartedAt) || maximum != nil && maximum.After(*value.CompletedAt) {
			return ErrValidation
		}
	}
	return nil
}

func validatePerformedAt(root Workout, performedAt *time.Time) error {
	if performedAt == nil || root.StartedAt == nil || performedAt.Before(*root.StartedAt) {
		return ErrValidation
	}
	if root.CompletedAt != nil && performedAt.After(*root.CompletedAt) {
		return ErrValidation
	}
	return nil
}

func validateListFilter(filter ListFilter) error {
	if filter.Limit < 1 || filter.Limit > 100 {
		return ErrValidation
	}
	switch filter.Status {
	case "", "planned", "in_progress", "completed", "cancelled":
	default:
		return ErrValidation
	}
	if filter.From != nil && filter.To != nil && !filter.From.Before(*filter.To) {
		return ErrValidation
	}
	if filter.ProgramID != "" && !id.ValidUUID(filter.ProgramID) || filter.ExerciseID != "" && !id.ValidUUID(filter.ExerciseID) {
		return ErrValidation
	}
	return nil
}

func applyOptional[T any](target **T, input Optional[T]) {
	if input.Set {
		*target = input.Value
	}
}

func applyOptionalText(target **string, input Optional[string], maximum int) error {
	if !input.Set {
		return nil
	}
	if input.Value == nil {
		*target = nil
		return nil
	}
	normalized, err := optionalText(input.Value, maximum)
	if err != nil {
		return err
	}
	*target = normalized
	return nil
}
