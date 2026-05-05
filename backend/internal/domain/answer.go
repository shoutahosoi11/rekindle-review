package domain

import "time"

type Answer struct {
	ID         string
	UserID     string
	QuestionID string
	UserAnswer string
	IsCorrect  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
