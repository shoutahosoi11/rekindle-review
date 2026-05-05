package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HighlightWriter interface {
	BulkUpsert(ctx context.Context, highlights []*Highlight) (saved int, err error)
	ListExistingContentHashesByUserID(ctx context.Context, userID uuid.UUID, hashes []string) ([]string, error)
	FindByUserIDAndContentHash(ctx context.Context, userID uuid.UUID, contentHash string) (*Highlight, error)
	UpdateExplanation(ctx context.Context, id, userID uuid.UUID, explanation *string) (*Highlight, error)
}

type HighlightReader interface {
	ListByUserIDAndASIN(ctx context.Context, userID uuid.UUID, asin string) ([]*Highlight, error)
	ListByUserIDAndBookMetadata(ctx context.Context, userID uuid.UUID, bookTitle, bookAuthor string) ([]*Highlight, error)
	ListBooksWithHighlightsByUserID(ctx context.Context, userID uuid.UUID) ([]*KindleBook, error)
}

type HighlightQuestionSyncReader interface {
	ListBookStockByUserID(ctx context.Context, userID uuid.UUID) ([]BookStock, error)
	ListUnusedHighlightsByBook(ctx context.Context, userID uuid.UUID, bookKey string, limit int) ([]*Highlight, error)
	ListUsedHighlightsWithUncoveredPerspectives(ctx context.Context, userID uuid.UUID, bookKey string, limit int) ([]*Highlight, error)
	ListQuestionGenerationCandidates(ctx context.Context, userID uuid.UUID, changedSince *time.Time) ([]QuestionGenerationBookCandidate, error)
	ListPendingHighlightsByBook(ctx context.Context, userID uuid.UUID, bookKey string, limit int) ([]*Highlight, error)
	MarkHighlightsProcessing(ctx context.Context, userID uuid.UUID, highlightIDs []uuid.UUID) error
	MarkHighlightPendingForQuestion(ctx context.Context, userID uuid.UUID, questionID uuid.UUID) (string, error)
}

type HighlightGenerationLifecycle interface {
	ListPendingUserStats(ctx context.Context) ([]PendingHighlightUserStat, error)
	ClaimPendingByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*Highlight, error)
	ClaimPendingByIDs(ctx context.Context, userID uuid.UUID, highlightIDs []uuid.UUID) ([]*Highlight, error)
	ListByIDs(ctx context.Context, userID uuid.UUID, highlightIDs []uuid.UUID) ([]*Highlight, error)
	RequeueStaleProcessing(ctx context.Context, cutoff time.Time) (int, error)
	MarkGenerationCompleted(ctx context.Context, highlightIDs []uuid.UUID) error
	MarkGenerationFailed(ctx context.Context, highlightIDs []uuid.UUID, lastError string, maxRetry int) error
}

type HighlightRepository interface {
	HighlightWriter
	HighlightReader
	HighlightQuestionSyncReader
	HighlightGenerationLifecycle
	RequeueStaleProcessingByUserID(ctx context.Context, userID uuid.UUID, cutoff time.Time) (int, error)
}

type HighlightImportRepository interface {
	HighlightWriter
	HighlightReader
}

type QuestionSyncHighlightRepository interface {
	HighlightQuestionSyncReader
	RequeueStaleProcessingByUserID(ctx context.Context, userID uuid.UUID, cutoff time.Time) (int, error)
}
