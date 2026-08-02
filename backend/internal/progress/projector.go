package progress

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

type RecordCandidateReader interface {
	RecordCandidates(context.Context, pgx.Tx, string) ([]workout.RecordCandidate, error)
}

type RecordProjector struct {
	repository *Repository
	candidates RecordCandidateReader
	newID      func() (string, error)
}

func NewRecordProjector(repository *Repository, candidates RecordCandidateReader) *RecordProjector {
	return &RecordProjector{repository: repository, candidates: candidates, newID: id.UUID}
}

func (p *RecordProjector) RebuildUser(ctx context.Context, tx pgx.Tx, userID string, now time.Time) error {
	candidates, err := p.candidates.RecordCandidates(ctx, tx, userID)
	if err != nil {
		return err
	}
	writes, err := discoverRecords(candidates, userID, now, p.newID)
	if err != nil {
		return err
	}
	return p.repository.ReplaceUser(ctx, tx, userID, writes)
}

func discoverRecords(candidates []workout.RecordCandidate, userID string, now time.Time, newID func() (string, error)) ([]recordWrite, error) {
	type maxima struct {
		weight, volume, e1rm *float64
		reps                 map[string]int16
	}
	byExercise := map[string]*maxima{}
	writes := []recordWrite{}
	add := func(c workout.RecordCandidate, kind string, value float64, formula *string) error {
		recordID, err := newID()
		if err != nil {
			return err
		}
		writes = append(writes, recordWrite{ID: recordID, UserID: userID, ExerciseID: c.ExerciseID, SetID: c.SetID, RecordType: kind, Value: math.Round(value*1000) / 1000, Formula: formula, AchievedAt: c.AchievedAt, CreatedAt: now.UTC()})
		return nil
	}
	for _, c := range candidates {
		current := byExercise[c.ExerciseID]
		if current == nil {
			current = &maxima{reps: map[string]int16{}}
			byExercise[c.ExerciseID] = current
		}
		if c.WeightKG != nil && (current.weight == nil || *c.WeightKG > *current.weight) {
			if err := add(c, "max_weight", *c.WeightKG, nil); err != nil {
				return nil, err
			}
			v := *c.WeightKG
			current.weight = &v
		}
		if c.WeightKey != nil && c.Repetitions != nil {
			old, exists := current.reps[*c.WeightKey]
			if !exists || *c.Repetitions > old {
				if err := add(c, "max_reps", float64(*c.Repetitions), nil); err != nil {
					return nil, err
				}
				current.reps[*c.WeightKey] = *c.Repetitions
			}
		}
		if c.WeightKG != nil && c.Repetitions != nil {
			volume := *c.WeightKG * float64(*c.Repetitions)
			if current.volume == nil || volume > *current.volume {
				if err := add(c, "max_set_volume", volume, nil); err != nil {
					return nil, err
				}
				current.volume = &volume
			}
			if value := workout.Estimated1RM(c.WeightKG, c.Repetitions, false); value != nil && (current.e1rm == nil || *value > *current.e1rm) {
				formula := "epley_v1_max_15_reps"
				if err := add(c, "estimated_1rm", *value, &formula); err != nil {
					return nil, err
				}
				v := *value
				current.e1rm = &v
			}
		}
	}
	return writes, nil
}
