package model

import "time"

type User struct {
	ID                         uint      `json:"id" gorm:"primaryKey"`
	Username                   string    `json:"username"`
	Email                      string    `json:"email"`
	Password                   string    `json:"password,omitempty"` // Exclude from JSON responses
	FirstName                  string    `json:"first_name"`
	LastName                   string    `json:"last_name"`
	InitialAssessmentCompleted bool      `json:"initial_assessment_completed" gorm:"default:false"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}
