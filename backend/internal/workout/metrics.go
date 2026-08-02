package workout

import "math"

func Volume(weightKG *float64, repetitions *int16, warmup bool) *float64 {
	if warmup || weightKG == nil || repetitions == nil {
		return nil
	}
	value := roundMetric(*weightKG * float64(*repetitions))
	return &value
}

func Estimated1RM(weightKG *float64, repetitions *int16, warmup bool) *float64 {
	if warmup || weightKG == nil || repetitions == nil || *weightKG <= 0 || *repetitions < 1 || *repetitions > 15 {
		return nil
	}
	value := roundMetric(*weightKG * (1 + float64(*repetitions)/30))
	return &value
}

func calculateMetrics(value *Workout) {
	value.CalculationVersion = CalculationVersion
	value.ExerciseCount = len(value.Exercises)
	value.SetCount = 0
	value.WorkingSetCount = 0
	value.VolumeKG = 0
	value.BestEstimated1RMKG = nil
	for exerciseIndex := range value.Exercises {
		exercise := &value.Exercises[exerciseIndex]
		exercise.VolumeKG = 0
		exercise.BestEstimated1RMKG = nil
		for setIndex := range exercise.Sets {
			set := &exercise.Sets[setIndex]
			set.VolumeKG, set.Estimated1RMKG = nil, nil
			if set.Status != "completed" {
				continue
			}
			value.SetCount++
			if !set.Warmup {
				value.WorkingSetCount++
			}
			set.VolumeKG = Volume(set.WeightKG, set.Repetitions, set.Warmup)
			set.Estimated1RMKG = Estimated1RM(set.WeightKG, set.Repetitions, set.Warmup)
			if set.VolumeKG != nil {
				exercise.VolumeKG += *set.VolumeKG
				value.VolumeKG += *set.VolumeKG
			}
			if set.Estimated1RMKG != nil {
				exercise.BestEstimated1RMKG = maxMetric(exercise.BestEstimated1RMKG, *set.Estimated1RMKG)
				value.BestEstimated1RMKG = maxMetric(value.BestEstimated1RMKG, *set.Estimated1RMKG)
			}
		}
		exercise.VolumeKG = roundMetric(exercise.VolumeKG)
	}
	value.VolumeKG = roundMetric(value.VolumeKG)
}

func maxMetric(current *float64, candidate float64) *float64 {
	if current != nil && *current >= candidate {
		return current
	}
	value := candidate
	return &value
}

func roundMetric(value float64) float64 {
	return math.Round(value*1000) / 1000
}
