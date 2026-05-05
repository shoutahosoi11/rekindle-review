package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type QuestionGenerationJobStatus string

const (
	JobStatusQueued        QuestionGenerationJobStatus = "queued"
	JobStatusProcessing    QuestionGenerationJobStatus = "processing"
	JobStatusCompleted     QuestionGenerationJobStatus = "completed"
	JobStatusFailed        QuestionGenerationJobStatus = "failed"
	JobStatusEnqueueFailed QuestionGenerationJobStatus = "enqueue_failed"
)

type QuestionGenerationJobReason string

const (
	JobReasonHighlightBatchThreshold QuestionGenerationJobReason = "highlight_batch_threshold"
	JobReasonAllUnansweredConsumed   QuestionGenerationJobReason = "all_unanswered_consumed"
	JobReasonManualSelection         QuestionGenerationJobReason = "manual_selection"
)

const (
	HighlightBatchThreshold = 10
	MinHighlightsForRefresh = 5
	MaxHighlightsPerJob     = 10
	JobMaxRetryCount        = 3
)

type QuestionGenerationJob struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	BookKey             string
	Status              QuestionGenerationJobStatus
	Reason              QuestionGenerationJobReason
	RetryCount          int
	LastError           string
	HighlightIDs        []uuid.UUID
	CreatedAt           time.Time
	ProcessingStartedAt *time.Time
	CompletedAt         *time.Time
	FailedAt            *time.Time
}

type CreateQuestionGenerationJobInput struct {
	UserID       uuid.UUID
	BookKey      string
	Reason       QuestionGenerationJobReason
	HighlightIDs []uuid.UUID
}

type QuestionGenerationJobRepository interface {
	Create(ctx context.Context, input CreateQuestionGenerationJobInput) (*QuestionGenerationJob, error)
	Get(ctx context.Context, jobID, userID uuid.UUID) (*QuestionGenerationJob, error)
	ListEnqueueFailedByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*QuestionGenerationJob, error)
	ClaimQueued(ctx context.Context, jobID, userID uuid.UUID) (*QuestionGenerationJob, bool, error)
	RequeueStaleProcessing(ctx context.Context, cutoff time.Time) (int, error)
	MarkQueued(ctx context.Context, jobID, userID uuid.UUID) error
	MarkCompleted(ctx context.Context, jobID, userID uuid.UUID) error
	MarkEnqueueFailed(ctx context.Context, jobID, userID uuid.UUID, lastError string) error
	RecordFailure(ctx context.Context, jobID, userID uuid.UUID, lastError string, maxRetry int) (*QuestionGenerationJob, error)
}

type QuestionGenerationTaskEnqueuer interface {
	EnqueueQuestionGeneration(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error
}
