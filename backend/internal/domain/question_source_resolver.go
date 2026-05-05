package domain

import "context"

type QuestionSourceResolver interface {
	ResolveHighlights(ctx context.Context, userID string, sourceType SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*Highlight, error)
}
