package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type ManualGenerationUsecase struct {
	jobRepo      domain.QuestionGenerationJobRepository
	budgetRepo   domain.QuestionBudgetRepository
	taskEnqueuer domain.QuestionGenerationTaskEnqueuer
	now          func() time.Time
}

func NewManualGenerationUsecase(
	jobRepo domain.QuestionGenerationJobRepository,
	budgetRepo domain.QuestionBudgetRepository,
	taskEnqueuer domain.QuestionGenerationTaskEnqueuer,
) *ManualGenerationUsecase {
	return &ManualGenerationUsecase{
		jobRepo:      jobRepo,
		budgetRepo:   budgetRepo,
		taskEnqueuer: taskEnqueuer,
		now:          time.Now,
	}
}

func (u *ManualGenerationUsecase) Generate(ctx context.Context, user *domain.User, bookKey string, highlightIDs []uuid.UUID) (*domain.QuestionGenerationJob, error) {
	if user == nil || strings.TrimSpace(bookKey) == "" || len(highlightIDs) < 5 {
		return nil, domain.ErrInvalidInput
	}

	job, err := u.jobRepo.Create(ctx, domain.CreateQuestionGenerationJobInput{
		UserID:       user.ID,
		BookKey:      bookKey,
		Reason:       domain.JobReasonManualSelection,
		HighlightIDs: highlightIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("manual generation usecase: create job: %w", err)
	}

	if _, err := u.budgetRepo.ReserveQuestions(ctx, user.ID, user.Plan, len(highlightIDs), u.now()); err != nil {
		if _, markErr := u.jobRepo.RecordFailure(ctx, job.ID, user.ID, err.Error(), 1); markErr != nil {
			return nil, fmt.Errorf("manual generation usecase: mark budget failure: %w", markErr)
		}
		return nil, err
	}

	if err := u.taskEnqueuer.EnqueueQuestionGeneration(ctx, job.ID, user.ID); err != nil {
		if markErr := u.jobRepo.MarkEnqueueFailed(ctx, job.ID, user.ID, err.Error()); markErr != nil {
			return nil, fmt.Errorf("manual generation usecase: mark enqueue failed: %w", markErr)
		}
	}

	return job, nil
}
