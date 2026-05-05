package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/middleware"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type stubHighlightRepository struct {
	bulkUpsertSaved  int
	bulkUpsertErr    error
	bulkUpsertCalled bool
	bulkUpsertInput  []*domain.Highlight
	persistedAt      time.Time
	existingHashes   []string
}

func (s *stubHighlightRepository) BulkUpsert(ctx context.Context, highlights []*domain.Highlight) (int, error) {
	s.bulkUpsertCalled = true
	s.bulkUpsertInput = highlights

	if s.bulkUpsertErr != nil {
		return 0, s.bulkUpsertErr
	}

	for i := 0; i < s.bulkUpsertSaved && i < len(highlights); i++ {
		highlights[i].ID = uuid.MustParse("99999999-9999-9999-9999-99999999999" + string(rune('1'+i)))
		highlights[i].CreatedAt = s.persistedAt
		highlights[i].UpdatedAt = s.persistedAt
	}

	return s.bulkUpsertSaved, nil
}

func (s *stubHighlightRepository) ListByUserIDAndASIN(ctx context.Context, userID uuid.UUID, asin string) ([]*domain.Highlight, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHighlightRepository) ListExistingContentHashesByUserID(ctx context.Context, userID uuid.UUID, hashes []string) ([]string, error) {
	return s.existingHashes, nil
}

func (s *stubHighlightRepository) FindByUserIDAndContentHash(ctx context.Context, userID uuid.UUID, contentHash string) (*domain.Highlight, error) {
	return &domain.Highlight{
		ID:          uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UserID:      userID,
		Content:     "existing",
		ContentHash: &contentHash,
		Source:      domain.HighlightSourcePaste,
	}, nil
}

func (s *stubHighlightRepository) ListByUserIDAndBookMetadata(ctx context.Context, userID uuid.UUID, bookTitle, bookAuthor string) ([]*domain.Highlight, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHighlightRepository) ListBooksWithHighlightsByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.KindleBook, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHighlightRepository) UpdateExplanation(ctx context.Context, id, userID uuid.UUID, explanation *string) (*domain.Highlight, error) {
	return nil, errors.New("not implemented")
}

func TestImportSharedReturnsSavedHighlight(t *testing.T) {
	e := echo.New()
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	repo := &stubHighlightRepository{
		bulkUpsertSaved: 1,
		persistedAt:     time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC),
	}
	handler := NewHighlightHandler(usecase.NewHighlightUsecase(repo), &stubUserUsecase{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return &domain.User{ID: userID, FirebaseUID: firebaseUID}, nil
		},
	})

	reqBody := `{"content":"  Focus is a superpower.  ","book_title":"Deep Work","book_author":"Cal Newport","source_app":"kindle","source_url":"https://read.amazon.com/notebook"}`
	req := httptest.NewRequest(http.MethodPost, "/highlights/share", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	if err := handler.ImportShared(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !repo.bulkUpsertCalled {
		t.Fatal("expected BulkUpsert to be called")
	}
	if len(repo.bulkUpsertInput) != 1 {
		t.Fatalf("expected one highlight to be persisted, got %d", len(repo.bulkUpsertInput))
	}

	var resp dto.ImportSharedHighlightResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Saved {
		t.Fatal("expected saved=true")
	}
	if resp.Duplicate {
		t.Fatal("expected duplicate=false")
	}
	if resp.Highlight == nil {
		t.Fatal("expected highlight response")
	}
	if resp.Highlight.Source != domain.HighlightSourceShare {
		t.Fatalf("unexpected source: %s", resp.Highlight.Source)
	}
	if resp.Highlight.SourceApp == nil || *resp.Highlight.SourceApp != "kindle" {
		t.Fatalf("unexpected source_app: %#v", resp.Highlight.SourceApp)
	}
	if resp.Highlight.SourceURL == nil || *resp.Highlight.SourceURL != "https://read.amazon.com/notebook" {
		t.Fatalf("unexpected source_url: %#v", resp.Highlight.SourceURL)
	}
}

func TestImportSharedReturnsBadRequestForEmptyContent(t *testing.T) {
	e := echo.New()
	handler := NewHighlightHandler(usecase.NewHighlightUsecase(&stubHighlightRepository{}), &stubUserUsecase{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return &domain.User{ID: uuid.New(), FirebaseUID: firebaseUID}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/highlights/share", strings.NewReader(`{"content":"   "}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	err := handler.ImportShared(c)
	if err == nil {
		t.Fatal("expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestImportPasteReturnsCreatedID(t *testing.T) {
	e := echo.New()
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	repo := &stubHighlightRepository{
		bulkUpsertSaved: 1,
		persistedAt:     time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
	}
	handler := NewHighlightHandler(usecase.NewHighlightUsecase(repo), &stubUserUsecase{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return &domain.User{ID: userID, FirebaseUID: firebaseUID}, nil
		},
	})

	reqBody := `{"content":"  Paste this idea.  ","book_title":"Notes","book_author":"Me","source_app":"web","source_url":"https://example.com/page"}`
	req := httptest.NewRequest(http.MethodPost, "/highlights/paste", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	if err := handler.ImportPaste(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp dto.ImportPastedHighlightResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("expected id")
	}
	if resp.Duplicate {
		t.Fatal("expected duplicated=false")
	}
	if len(repo.bulkUpsertInput) != 1 {
		t.Fatalf("expected one highlight to be persisted, got %d", len(repo.bulkUpsertInput))
	}
	if repo.bulkUpsertInput[0].Source != domain.HighlightSourcePaste {
		t.Fatalf("unexpected source: %s", repo.bulkUpsertInput[0].Source)
	}
}

func TestImportPasteReturnsBadRequestForInvalidSourceURL(t *testing.T) {
	e := echo.New()
	handler := NewHighlightHandler(usecase.NewHighlightUsecase(&stubHighlightRepository{}), &stubUserUsecase{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return &domain.User{ID: uuid.New(), FirebaseUID: firebaseUID}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/highlights/paste", strings.NewReader(`{"content":"hello","source_url":"javascript:alert(1)"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	err := handler.ImportPaste(c)
	if err == nil {
		t.Fatal("expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestCheckExistingHashesReturnsMatches(t *testing.T) {
	e := echo.New()
	repo := &stubHighlightRepository{
		existingHashes: []string{
			"hash-1",
			"hash-3",
		},
	}
	handler := NewHighlightHandler(usecase.NewHighlightUsecase(repo), &stubUserUsecase{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return &domain.User{ID: uuid.New(), FirebaseUID: firebaseUID}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/highlights/sync/check", strings.NewReader(`{"hashes":["hash-1","hash-2","hash-3"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	if err := handler.CheckExistingHashes(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp dto.CheckHighlightHashesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.ExistingHashes) != 2 {
		t.Fatalf("expected 2 hashes, got %d", len(resp.ExistingHashes))
	}
}
