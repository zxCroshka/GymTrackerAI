export interface WeightSummary {
  current_kg: number | null;
  change_7d_kg: number | null;
  change_30d_kg: number | null;
  moving_average_7d_kg: number | null;
}

export interface PersonalRecord {
  id: string;
  exercise_name: string;
  record_type: "max_weight" | "max_reps" | "max_set_volume" | "estimated_1rm";
  value: number;
  unit: string;
  weight_kg: number | null;
  achieved_at: string;
}

export interface ProgressDashboard {
  as_of: string;
  timezone: string;
  week_start_at: string;
  week_end_at: string;
  weight: WeightSummary;
  workouts_this_week: number;
  total_volume_kg: number;
  weekly_volume_kg: number;
  training_streak_weeks: number;
  new_achievements: PersonalRecord[];
  next_planned_workout: { id: string; name: string; scheduled_at: string } | null;
  calculation_version: string;
}
