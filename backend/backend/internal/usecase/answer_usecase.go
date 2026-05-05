package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type AnswerUsecase struct {
	answerRepo   domain.AnswerRepository
	questionRepo domain.AnswerQuestionRepository
}

func NewAnswerUsecase(
	answerRepo domain.AnswerRepository,
	questionRepo domain.AnswerQuestionRepository,
) *AnswerUsecase {
	return &AnswerUsecase{
		answerRepo:   answerRepo,
		questionRepo: questionRepo,
	}
}

type SubmitAnswerInput struct {
	UserID     string
	QuestionID string
	UserAnswer string
	UserPlan   string
}

type SubmitAnswerResult struct {
	IsCorrect     bool
	CorrectAnswer string
	Explanation   string
}

func (u *AnswerUsecase) SubmitAnswer(ctx context.Context, input SubmitAnswerInput) (*SubmitAnswerResult, error) {
	q, _, _, err := u.questionRepo.FindByID(ctx, input.QuestionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("answer usecase: question not found: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("answer usecase: get question: %w", err)
	}

	isCorrect := q.IsCorrect(input.UserAnswer)
	upsertInput := domain.AnswerUpsertInput{
		UserID:     input.UserID,
		QuestionID: input.QuestionID,
		UserAnswer: input.UserAnswer,
		IsCorrect:  isCorrect,
	}
	if _, err := u.answerRepo.UpsertAndUpdateStats(ctx, upsertInput); err != nil {
		return nil, fmt.Errorf("answer usecase: upsert answer: %w", err)
	}

	return &SubmitAnswerResult{
		IsCorrect:     isCorrect,
		CorrectAnswer: q.CorrectAnswer,
		Explanation:   q.Explanation,
	}, nil
}
