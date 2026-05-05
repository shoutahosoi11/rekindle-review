package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type questionSourceResolver struct {
	highlightRepo highlightSourceReader
}

type highlightSourceReader interface {
	ListByUserIDAndASIN(ctx context.Context, userID uuid.UUID, asin string) ([]*domain.Highlight, error)
	ListByUserIDAndBookMetadata(ctx context.Context, userID uuid.UUID, bookTitle, bookAuthor string) ([]*domain.Highlight, error)
}

func NewQuestionSourceResolver(highlightRepo highlightSourceReader) domain.QuestionSourceResolver {
	return &questionSourceResolver{
		highlightRepo: highlightRepo,
	}
}

func (r *questionSourceResolver) ResolveHighlights(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, domain.ErrInvalidSourceType
	}

	switch sourceType {
	case domain.SourceTypeKindleBook:
		return r.resolveKindleBookHighlights(ctx, userUUID, sourceID, bookTitle, bookAuthor)
	default:
		return nil, domain.ErrInvalidSourceType
	}
}

func (r *questionSourceResolver) resolveKindleBookHighlights(ctx context.Context, userUUID uuid.UUID, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
	asin := strings.TrimSpace(sourceID)
	normalizedTitle := strings.TrimSpace(bookTitle)
	normalizedAuthor := strings.TrimSpace(bookAuthor)

	if asin == "" && normalizedTitle == "" {
		return nil, domain.ErrInvalidSourceType
	}

	merged := make([]*domain.Highlight, 0)
	seen := make(map[uuid.UUID]struct{})

	if asin != "" {
		highlights, err := r.highlightRepo.ListByUserIDAndASIN(ctx, userUUID, asin)
		if err != nil {
			return nil, fmt.Errorf("question source resolver: list highlights by asin: %w", err)
		}
		for _, highlight := range highlights {
			if _, ok := seen[highlight.ID]; ok {
				continue
			}
			seen[highlight.ID] = struct{}{}
			merged = append(merged, highlight)
		}
	}

	if normalizedTitle == "" {
		return merged, nil
	}

	metadataHighlights, metadataErr := r.highlightRepo.ListByUserIDAndBookMetadata(ctx, userUUID, normalizedTitle, normalizedAuthor)
	if metadataErr != nil {
		return nil, fmt.Errorf("question source resolver: list highlights by metadata: %w", metadataErr)
	}
	for _, highlight := range metadataHighlights {
		if _, ok := seen[highlight.ID]; ok {
			continue
		}
		seen[highlight.ID] = struct{}{}
		merged = append(merged, highlight)
	}

	return merged, nil
}
