package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/middleware"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type stubQuestionUsecase struct {
	saveQuestion func(ctx context.Context, userID string, questionID string, note string) error
}

func (s *stubQuestionUsecase) ListQuestions(ctx context.Context, creatorID string, limit int) ([]*domain.Question, error) {
	return nil, nil
}

func (s *stubQuestionUsecase) ListSavedQuestions(ctx context.Context, userID string, limit int) ([]*domain.SavedQuestion, error) {
	return nil, nil
}

func (s *stubQuestionUsecase) ListIncorrectQuestions(ctx context.Context, userID string, limit int) ([]*domain.IncorrectQuestion, error) {
	return nil, nil
}

func (s *stubQuestionUsecase) ListPreparedQuestions(ctx context.Context, input domain.GenerateQuestionsInput) ([]*domain.Question, error) {
	return nil, nil
}

func (s *stubQuestionUsecase) SaveQuestion(ctx context.Context, userID string, questionID string, note string) error {
	if s.saveQuestion == nil {
		return nil
	}
	return s.saveQuestion(ctx, userID, questionID, note)
}

type stubQuestionSyncUsecase struct{}

func (s *stubQuestionSyncUsecase) SyncQuestionStock(ctx context.Context, user *domain.User) (*usecase.SyncQuestionStockResult, error) {
	return &usecase.SyncQuestionStockResult{}, nil
}

func (s *stubQuestionSyncUsecase) EvaluateBookAfterAnswer(ctx context.Context, user *domain.User, questionID string) error {
	return nil
}

func TestSaveQuestionTrimsNoteBeforeSaving(t *testing.T) {
	e := echo.New()
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	var savedNote string
	handler := NewQuestionHandler(&stubQuestionUsecase{
		saveQuestion: func(ctx context.Context, userID string, questionID string, note string) error {
			savedNote = note
			return nil
		},
	}, &stubQuestionSyncUsecase{}, questionHandlerUserUsecase(userID))

	req := httptest.NewRequest(http.MethodPost, "/questions/q-1/save", strings.NewReader(`{"note":"  keep this  "}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")
	c.SetParamNames("id")
	c.SetParamValues("q-1")

	if err := handler.SaveQuestion(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if savedNote != "keep this" {
		t.Fatalf("expected trimmed note to be saved, got %q", savedNote)
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response["note"] != savedNote {
		t.Fatalf("response note %q differs from saved note %q", response["note"], savedNote)
	}
}

func TestValidateQuestionSourceIDReturnsInvalidInputForEmptyID(t *testing.T) {
	err := validateQuestionSource(domain.SourceTypeKindleBook, "  ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func questionHandlerUserUsecase(userID uuid.UUID) *stubUserUsecase {
	return &stubUserUsecase{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return &domain.User{ID: userID, FirebaseUID: firebaseUID}, nil
		},
	}
}

func TestListPreparedRejectsNonNumericQuestionCount(t *testing.T) {
	e := echo.New()
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	handler := NewQuestionHandler(&stubQuestionUsecase{}, &stubQuestionSyncUsecase{}, questionHandlerUserUsecase(userID))

	req := httptest.NewRequest(http.MethodGet, "/questions/prepared?source_type=kindle_book&source_id=book-1&question_count=abc", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	err := handler.ListPrepared(c)
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", httpErr.Code)
	}
}

func TestListPreparedRejectsNegativeQuestionCount(t *testing.T) {
	e := echo.New()
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	handler := NewQuestionHandler(&stubQuestionUsecase{}, &stubQuestionSyncUsecase{}, questionHandlerUserUsecase(userID))

	req := httptest.NewRequest(http.MethodGet, "/questions/prepared?source_type=kindle_book&source_id=book-1&question_count=-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	err := handler.ListPrepared(c)
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", httpErr.Code)
	}
}

func TestListPreparedAllowsZeroQuestionCount(t *testing.T) {
	e := echo.New()
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	handler := NewQuestionHandler(&stubQuestionUsecase{}, &stubQuestionSyncUsecase{}, questionHandlerUserUsecase(userID))

	req := httptest.NewRequest(http.MethodGet, "/questions/prepared?source_type=kindle_book&source_id=book-1&question_count=0", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	err := handler.ListPrepared(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
