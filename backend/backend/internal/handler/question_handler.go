package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

const (
	defaultQuestionListLimit    = 50
	defaultQuestionHistoryLimit = 100
)

type QuestionHandler struct {
	questionUsecase     QuestionUsecase
	questionSyncUsecase QuestionSyncUsecase
	manualUsecase       ManualGenerationUsecase
	userUsecase         usecase.UserUsecaseInterface
}

type QuestionUsecase interface {
	ListQuestions(ctx context.Context, creatorID string, limit int) ([]*domain.Question, error)
	ListSavedQuestions(ctx context.Context, userID string, limit int) ([]*domain.SavedQuestion, error)
	ListIncorrectQuestions(ctx context.Context, userID string, limit int) ([]*domain.IncorrectQuestion, error)
	ListPreparedQuestions(ctx context.Context, input domain.GenerateQuestionsInput) ([]*domain.Question, error)
	SaveQuestion(ctx context.Context, userID string, questionID string, note string) error
}

type QuestionSyncUsecase interface {
	SyncQuestionStock(ctx context.Context, user *domain.User) (*usecase.SyncQuestionStockResult, error)
	EvaluateBookAfterAnswer(ctx context.Context, user *domain.User, questionID string) error
}

type ManualGenerationUsecase interface {
	Generate(ctx context.Context, user *domain.User, bookKey string, highlightIDs []uuid.UUID) (*domain.QuestionGenerationJob, error)
}

func NewQuestionHandler(
	qu QuestionUsecase,
	questionSyncUsecase QuestionSyncUsecase,
	userUsecase usecase.UserUsecaseInterface,
	manualUsecases ...ManualGenerationUsecase,
) *QuestionHandler {
	var manualUsecase ManualGenerationUsecase
	if len(manualUsecases) > 0 {
		manualUsecase = manualUsecases[0]
	}
	return &QuestionHandler{
		questionUsecase:     qu,
		questionSyncUsecase: questionSyncUsecase,
		manualUsecase:       manualUsecase,
		userUsecase:         userUsecase,
	}
}

func (h *QuestionHandler) List(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	questions, err := h.questionUsecase.ListQuestions(c.Request().Context(), user.ID.String(), defaultQuestionListLimit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	responses := make([]dto.QuestionResponse, 0, len(questions))
	for _, q := range questions {
		responses = append(responses, dto.ToQuestionResponse(q))
	}

	return c.JSON(http.StatusOK, responses)
}

func (h *QuestionHandler) ListSaved(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	savedQuestions, err := h.questionUsecase.ListSavedQuestions(c.Request().Context(), user.ID.String(), defaultQuestionHistoryLimit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	responses := make([]dto.SavedQuestionResponse, 0, len(savedQuestions))
	for _, q := range savedQuestions {
		responses = append(responses, dto.ToSavedQuestionResponse(q))
	}

	return c.JSON(http.StatusOK, responses)
}

func (h *QuestionHandler) ListIncorrect(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	incorrectQuestions, err := h.questionUsecase.ListIncorrectQuestions(c.Request().Context(), user.ID.String(), defaultQuestionHistoryLimit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	responses := make([]dto.IncorrectQuestionResponse, 0, len(incorrectQuestions))
	for _, q := range incorrectQuestions {
		responses = append(responses, dto.ToIncorrectQuestionResponse(q))
	}

	return c.JSON(http.StatusOK, responses)
}

func (h *QuestionHandler) ListPrepared(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	sourceType := domain.SourceType(strings.TrimSpace(c.QueryParam("source_type")))
	sourceID := strings.TrimSpace(c.QueryParam("source_id"))
	if sourceID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "source_id is required")
	}
	if err := validateQuestionSource(sourceType, sourceID); err != nil {
		if errors.Is(err, domain.ErrInvalidSourceType) {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid source type")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "invalid source id")
	}

	questionCount := 0
	if rawLimit := strings.TrimSpace(c.QueryParam("question_count")); rawLimit != "" {
		parsedLimit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsedLimit < 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid question_count")
		}
		questionCount = parsedLimit
	}
	startIndex, err := parseOptionalNonNegativeInt(c.QueryParam("highlight_start_index"), "highlight_start_index")
	if err != nil {
		return err
	}
	endIndex, err := parseOptionalNonNegativeInt(c.QueryParam("highlight_end_index"), "highlight_end_index")
	if err != nil {
		return err
	}

	input := domain.GenerateQuestionsInput{
		CreatorID:           user.ID.String(),
		SourceType:          sourceType,
		SourceID:            sourceID,
		BookTitle:           strings.TrimSpace(c.QueryParam("book_title")),
		BookAuthor:          strings.TrimSpace(c.QueryParam("book_author")),
		QuestionCount:       questionCount,
		HighlightStartIndex: startIndex,
		HighlightEndIndex:   endIndex,
	}

	questions, err := h.questionUsecase.ListPreparedQuestions(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidSourceType) {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid source type")
		}
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "source not found")
		}
		if errors.Is(err, domain.ErrQuestionsPreparing) {
			return echo.NewHTTPError(http.StatusConflict, "questions are still preparing")
		}
		if errors.Is(err, domain.ErrQuestionGenerationFailed) {
			return echo.NewHTTPError(http.StatusConflict, "question generation failed")
		}
		if errors.Is(err, domain.ErrSourceTextUnavailable) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "source text is unavailable")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	responses := make([]dto.QuestionResponse, 0, len(questions))
	for _, q := range questions {
		responses = append(responses, dto.ToQuestionResponse(q))
	}

	return c.JSON(http.StatusOK, responses)
}

func (h *QuestionHandler) SyncStock(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	result, err := h.questionSyncUsecase.SyncQuestionStock(c.Request().Context(), user)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	response := dto.SyncQuestionStockResponse{
		Books:                  make([]dto.SyncQuestionStockBookResponse, 0, len(result.Books)),
		QueuedCount:            result.QueuedCount,
		SkippedDueToDailyLimit: result.SkippedDueToDailyLimit,
	}
	for _, book := range result.Books {
		response.Books = append(response.Books, dto.SyncQuestionStockBookResponse{
			BookKey:    book.BookKey,
			BookTitle:  book.BookTitle,
			BookAuthor: book.BookAuthor,
			Stock:      book.Stock,
			Target:     book.Target,
			Preparing:  book.Preparing,
		})
	}

	return c.JSON(http.StatusOK, response)
}

func (h *QuestionHandler) ManualGenerate(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}
	if h.manualUsecase == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "manual generation is unavailable")
	}

	req := new(dto.ManualGenerateQuestionRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if len(req.HighlightIDs) < 5 {
		return echo.NewHTTPError(http.StatusBadRequest, "minimum 5 highlights required")
	}

	highlightIDs := make([]uuid.UUID, 0, len(req.HighlightIDs))
	for _, rawID := range req.HighlightIDs {
		highlightID, parseErr := uuid.Parse(strings.TrimSpace(rawID))
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid highlight id")
		}
		highlightIDs = append(highlightIDs, highlightID)
	}

	job, err := h.manualUsecase.Generate(c.Request().Context(), user, strings.TrimSpace(req.BookKey), highlightIDs)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
		}
		if errors.Is(err, domain.ErrQuestionBudgetExceeded) {
			return echo.NewHTTPError(http.StatusPaymentRequired, "question budget exceeded")
		}
		if errors.Is(err, domain.ErrAlreadyExists) {
			return echo.NewHTTPError(http.StatusConflict, "generation job already exists")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusAccepted, dto.ManualGenerateQuestionResponse{JobID: job.ID.String()})
}

func parseOptionalNonNegativeInt(rawValue string, name string) (int, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(normalized)
	if err != nil || parsed < 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid "+name)
	}
	return parsed, nil
}

func (h *QuestionHandler) SaveQuestion(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	questionID := strings.TrimSpace(c.Param("id"))
	if questionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "question id is required")
	}

	req := new(dto.SaveQuestionRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	note := strings.TrimSpace(req.Note)
	if err := h.questionUsecase.SaveQuestion(c.Request().Context(), user.ID.String(), questionID, note); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "question not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.SaveQuestionResponse{
		QuestionID: questionID,
		Note:       note,
		Saved:      true,
	})
}

func validateQuestionSource(sourceType domain.SourceType, sourceID string) error {
	switch sourceType {
	case domain.SourceTypeKindleBook:
		if strings.TrimSpace(sourceID) == "" {
			return domain.ErrInvalidInput
		}
		return nil
	default:
		return domain.ErrInvalidSourceType
	}
}

func (h *QuestionHandler) currentUser(c echo.Context) (*domain.User, error) {
	return resolveCurrentUser(c, h.userUsecase, "question")
}
