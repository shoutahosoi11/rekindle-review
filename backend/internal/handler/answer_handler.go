package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type AnswerHandler struct {
	answerUsecase       AnswerUsecase
	userUsecase         usecase.UserUsecaseInterface
	questionSyncUsecase QuestionSyncUsecase
}

type AnswerUsecase interface {
	SubmitAnswer(ctx context.Context, input usecase.SubmitAnswerInput) (*usecase.SubmitAnswerResult, error)
}

func NewAnswerHandler(au AnswerUsecase, userUsecase usecase.UserUsecaseInterface, questionSyncUsecase QuestionSyncUsecase) *AnswerHandler {
	return &AnswerHandler{
		answerUsecase:       au,
		userUsecase:         userUsecase,
		questionSyncUsecase: questionSyncUsecase,
	}
}

func (h *AnswerHandler) SubmitAnswer(c echo.Context) error {
	user, err := resolveCurrentUser(c, h.userUsecase, "answer")
	if err != nil {
		return err
	}

	questionID := strings.TrimSpace(c.Param("id"))
	if questionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "question id is required")
	}

	req := new(dto.SubmitAnswerRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.UserAnswer == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_answer is required")
	}

	input := usecase.SubmitAnswerInput{
		UserID:     user.ID.String(),
		QuestionID: questionID,
		UserAnswer: req.UserAnswer,
		UserPlan:   user.Plan,
	}

	result, err := h.answerUsecase.SubmitAnswer(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "question not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
	if h.questionSyncUsecase != nil {
		if err := h.questionSyncUsecase.EvaluateBookAfterAnswer(c.Request().Context(), user, questionID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
		}
	}

	return c.JSON(http.StatusOK, dto.SubmitAnswerResponse{
		IsCorrect:     result.IsCorrect,
		CorrectAnswer: result.CorrectAnswer,
		Explanation:   result.Explanation,
	})
}
