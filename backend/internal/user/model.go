package user

import "time"

type Profile struct {
	UserID            string    `json:"user_id"`
	Name              *string   `json:"name"`
	Sex               *string   `json:"sex"`
	BirthDate         *string   `json:"birth_date"`
	HeightCM          *float64  `json:"height_cm"`
	Goal              *string   `json:"goal"`
	ExperienceLevel   *string   `json:"experience_level"`
	TrainingFrequency *int16    `json:"training_frequency"`
	Timezone          string    `json:"timezone"`
	UnitSystem        string    `json:"unit_system"`
	SleepHoursAverage *float64  `json:"sleep_hours_average"`
	Notes             []string  `json:"notes"`
	Version           int64     `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type databaseProfile struct {
	UserID            string
	Name              *string
	Sex               *string
	BirthDate         *time.Time
	HeightCM          *float64
	Goal              *string
	ExperienceLevel   *string
	TrainingFrequency *int16
	Timezone          string
	UnitSystem        string
	SleepHoursAverage *float64
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Notes             []string
}

func profileFromDatabase(value databaseProfile) Profile {
	var birthDate *string
	if value.BirthDate != nil {
		formatted := value.BirthDate.Format(time.DateOnly)
		birthDate = &formatted
	}
	notes := value.Notes
	if notes == nil {
		notes = []string{}
	}
	return Profile{
		UserID: value.UserID, Name: value.Name, Sex: value.Sex, BirthDate: birthDate,
		HeightCM: value.HeightCM, Goal: value.Goal, ExperienceLevel: value.ExperienceLevel,
		TrainingFrequency: value.TrainingFrequency, Timezone: value.Timezone, UnitSystem: value.UnitSystem,
		SleepHoursAverage: value.SleepHoursAverage, Notes: notes, Version: value.Version,
		CreatedAt: value.CreatedAt.UTC(), UpdatedAt: value.UpdatedAt.UTC(),
	}
}
