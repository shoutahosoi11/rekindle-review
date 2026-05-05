package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type mockAnswerRepository struct {
	upsertInput *domain.AnswerUpsertInput
}

func (m *mockAnswerRepository) Upsert(ctx context.Context, input domain.AnswerUpsertInput) (*domain.Answer, error) {
	m.upsertInput = &input
	return &domain.Answer{IsCorrect: input.IsCorrect}, nil
}

func (m *mockAnswerRepository) UpsertAndUpdateStats(ctx context.Context, input domain.AnswerUpsertInput) (*domain.Answer, error) {
	m.upsertInput = &input
	return &domain.Answer{IsCorrect: input.IsCorrect}, nil
}

type mockAnswerQuestionRepository struct {
	question *domain.Question
}

func (m *mockAnswerQuestionRepository) FindByID(ctx context.Context, id string) (*domain.Question, *domain.QuestionMeta, *domain.QuestionStats, error) {
	return m.question, nil, nil, nil
}

func (m *mockAnswerQuestionRepository) EnqueueRegeneration(ctx context.Context, userID string, highlightID uuid.UUID, questionID string) error {
	return nil
}

func TestSubmitAnswerUsesMultipleChoiceCorrectAnswerOnly(t *testing.T) {
	answerRepo := &mockAnswerRepository{}
	questionRepo := &mockAnswerQuestionRepository{
		question: &domain.Question{
			ID:            uuid.NewString(),
			QuestionType:  domain.QuestionTypeMultipleChoice,
			CorrectAnswer: "A",
			Explanation:   "生成時の解説",
		},
	}
	uc := NewAnswerUsecase(answerRepo, questionRepo)

	result, err := uc.SubmitAnswer(context.Background(), SubmitAnswerInput{
		UserID:     uuid.NewString(),
		QuestionID: questionRepo.question.ID,
		UserAnswer: "A",
		UserPlan:   "pro",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer failed: %v", err)
	}

	if !result.IsCorrect {
		t.Fatal("expected answer to be correct")
	}
	if result.Explanation != "生成時の解説" {
		t.Fatalf("expected generated explanation to be returned, got %q", result.Explanation)
	}
	if answerRepo.upsertInput == nil || !answerRepo.upsertInput.IsCorrect {
		t.Fatalf("expected correct answer upsert, got %#v", answerRepo.upsertInput)
	}
}
