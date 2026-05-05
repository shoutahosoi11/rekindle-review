package domain

import "context"

type AnswerUpsertInput struct {
	UserID     string
	QuestionID string
	UserAnswer string
	IsCorrect  bool
}

type AnswerRepository interface {
	Upsert(ctx context.Context, input AnswerUpsertInput) (*Answer, error)
	UpsertAndUpdateStats(ctx context.Context, input AnswerUpsertInput) (*Answer, error)
}
