package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type mockHighlightRepository struct {
	listByUserIDAndASIN    func(ctx context.Context, userID uuid.UUID, asin string) ([]*domain.Highlight, error)
	listByUserIDAndBookMet func(ctx context.Context, userID uuid.UUID, bookTitle, bookAuthor string) ([]*domain.Highlight, error)
}

func (m *mockHighlightRepository) ListByUserIDAndASIN(ctx context.Context, userID uuid.UUID, asin string) ([]*domain.Highlight, error) {
	if m.listByUserIDAndASIN == nil {
		return make([]*domain.Highlight, 0), nil
	}
	return m.listByUserIDAndASIN(ctx, userID, asin)
}

func (m *mockHighlightRepository) ListByUserIDAndBookMetadata(ctx context.Context, userID uuid.UUID, bookTitle, bookAuthor string) ([]*domain.Highlight, error) {
	if m.listByUserIDAndBookMet == nil {
		return make([]*domain.Highlight, 0), nil
	}
	return m.listByUserIDAndBookMet(ctx, userID, bookTitle, bookAuthor)
}

func TestQuestionSourceResolver_ResolveHighlightsFromKindleBook(t *testing.T) {
	userID := uuid.New()
	asin := "B00TEST"

	resolver := usecase.NewQuestionSourceResolver(
		&mockHighlightRepository{
			listByUserIDAndASIN: func(ctx context.Context, requestUserID uuid.UUID, requestASIN string) ([]*domain.Highlight, error) {
				if requestUserID != userID {
					t.Fatalf("unexpected user id: %s", requestUserID)
				}
				if requestASIN != asin {
					t.Fatalf("unexpected asin: %s", requestASIN)
				}

				return []*domain.Highlight{
					{ID: uuid.New(), Content: "1つ目のハイライト"},
					{ID: uuid.New(), Content: "2つ目のハイライト"},
				}, nil
			},
		},
	)

	highlights, err := resolver.ResolveHighlights(context.Background(), userID.String(), domain.SourceTypeKindleBook, asin, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(highlights) != 2 {
		t.Fatalf("expected 2 highlights, got %d", len(highlights))
	}
}

func TestQuestionSourceResolver_ResolveHighlightsFallsBackToBookMetadata(t *testing.T) {
	userID := uuid.New()

	resolver := usecase.NewQuestionSourceResolver(
		&mockHighlightRepository{
			listByUserIDAndASIN: func(ctx context.Context, requestUserID uuid.UUID, requestASIN string) ([]*domain.Highlight, error) {
				return []*domain.Highlight{}, nil
			},
			listByUserIDAndBookMet: func(ctx context.Context, requestUserID uuid.UUID, bookTitle, bookAuthor string) ([]*domain.Highlight, error) {
				if requestUserID != userID {
					t.Fatalf("unexpected user id: %s", requestUserID)
				}
				if bookTitle != "テスト本" {
					t.Fatalf("unexpected title: %s", bookTitle)
				}
				if bookAuthor != "著者A" {
					t.Fatalf("unexpected author: %s", bookAuthor)
				}

				return []*domain.Highlight{
					{ID: uuid.New(), Content: "metadata 1"},
					{ID: uuid.New(), Content: "metadata 2"},
				}, nil
			},
		},
	)

	highlights, err := resolver.ResolveHighlights(context.Background(), userID.String(), domain.SourceTypeKindleBook, "B00FALLBACK", "テスト本", "著者A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(highlights) != 2 {
		t.Fatalf("expected 2 highlights, got %d", len(highlights))
	}
}

func TestQuestionSourceResolver_ResolveHighlightsRejectsUnsupportedSourceType(t *testing.T) {
	userID := uuid.New()

	resolver := usecase.NewQuestionSourceResolver(&mockHighlightRepository{})

	_, err := resolver.ResolveHighlights(context.Background(), userID.String(), domain.SourceType("highlight"), "source", "", "")
	if !errors.Is(err, domain.ErrInvalidSourceType) {
		t.Fatalf("expected ErrInvalidSourceType, got %v", err)
	}
}
