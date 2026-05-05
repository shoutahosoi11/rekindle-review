package persistence

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

func TestShouldRetryBulkUpsertLegacy(t *testing.T) {
	t.Run("retries when source_app column is missing", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", &pq.Error{
			Code:    "42703",
			Message: `column "source_app" of relation "highlights" does not exist`,
		})

		if !shouldRetryBulkUpsertLegacy(err) {
			t.Fatal("expected true for wrapped pq error")
		}
	})

	t.Run("retries when pq error mentions source_app", func(t *testing.T) {
		err := &pq.Error{
			Code:    "42703",
			Message: `column "source_app" of relation "highlights" does not exist`,
		}

		if !shouldRetryBulkUpsertLegacy(err) {
			t.Fatal("expected true")
		}
	})

	t.Run("does not retry unrelated missing column", func(t *testing.T) {
		err := &pq.Error{
			Code:    "42703",
			Message: `column "book_title" of relation "highlights" does not exist`,
		}

		if shouldRetryBulkUpsertLegacy(err) {
			t.Fatal("expected false")
		}
	})
}

func TestBuildLegacyHighlightBulkUpsert(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contentHash := "hash-1"
	sourceApp := "kindle"
	sourceURL := "https://a.co/example"

	query, args, hashIndex, err := buildLegacyHighlightBulkUpsert([]*domain.Highlight{
		{
			UserID:      userID,
			Content:     "hello",
			Source:      domain.HighlightSourceShare,
			ContentHash: &contentHash,
			SourceApp:   &sourceApp,
			SourceURL:   &sourceURL,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(query, "source_app") || strings.Contains(query, "source_url") {
		t.Fatalf("legacy query should not reference new columns: %s", query)
	}
	if len(args) != 11 {
		t.Fatalf("expected 11 args, got %d", len(args))
	}
	if len(hashIndex[contentHash]) != 1 {
		t.Fatalf("expected hash index to contain one highlight, got %d", len(hashIndex[contentHash]))
	}
}
