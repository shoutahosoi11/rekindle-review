package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HighlightImportQueueStatus string

const (
	ImportQueueStatusQueued     HighlightImportQueueStatus = "queued"
	ImportQueueStatusProcessing HighlightImportQueueStatus = "processing"
	ImportQueueStatusCompleted  HighlightImportQueueStatus = "completed"
	ImportQueueStatusFailed     HighlightImportQueueStatus = "failed"
)

const (
	ImportQueueSourceKindle = "kindle"
	ImportQueueMaxRetry     = 3
	ImportQueueStaleTimeout = 10 * time.Minute
)

type HighlightImportQueue struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	Source              string
	RawPayload          []byte
	Status              HighlightImportQueueStatus
	RetryCount          int
	LastError           string
	CreatedAt           time.Time
	ProcessingStartedAt *time.Time
	CompletedAt         *time.Time
	FailedAt            *time.Time
}

type HighlightImportQueueRepository interface {
	Enqueue(ctx context.Context, userID uuid.UUID, source string, payload []byte) (uuid.UUID, error)
	DequeueBatch(ctx context.Context, limit int) ([]*HighlightImportQueue, error)
	ClaimProcessing(ctx context.Context, id uuid.UUID) (bool, error)
	MarkCompleted(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error
	RequeueWithRetry(ctx context.Context, id uuid.UUID, errMsg string) error
	RequeueStale(ctx context.Context, cutoff time.Time) (int, error)
}

// HighlightImportJobTrigger abstracts Cloud Run Job execution.
// usecase 層が infrastructure に依存しないよう抽象化。
// HIGHLIGHT_IMPORT_JOB_NAME 環境変数が未設定の場合は no-op 実装を使う。
type HighlightImportJobTrigger interface {
	TriggerHighlightImportJob(ctx context.Context) error
}
