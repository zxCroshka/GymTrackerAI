export interface Profile {
  user_id: string;
  name: string | null;
  sex: "male" | "female" | "other" | "prefer_not_to_say" | null;
  birth_date: string | null;
  height_cm: number | null;
  goal: "muscle_gain" | "weight_loss" | "recomposition" | "strength" | "maintenance" | null;
  experience_level: "beginner" | "intermediate" | "advanced" | null;
  training_frequency: number | null;
  timezone: string;
  unit_system: "metric" | "imperial";
  sleep_hours_average: number | null;
  notes: string[];
  version: number;
  created_at: string;
  updated_at: string;
}

export interface ProfileResource {
  profile: Profile;
  etag: string;
}

export interface ProfileImportResult {
  profile: Profile;
  initial_measurement_id: string | null;
}
