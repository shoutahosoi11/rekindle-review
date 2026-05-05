package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type TokenUsecase interface {
	Award(ctx context.Context, user *domain.User) (*domain.QuestionTokenBalance, error)
	Balance(ctx context.Context, user *domain.User) (*domain.QuestionTokenBalance, error)
}

type TokenHandler struct {
	tokenUsecase TokenUsecase
	userUsecase  usecase.UserUsecaseInterface
}

func NewTokenHandler(tokenUsecase TokenUsecase, userUsecase usecase.UserUsecaseInterface) *TokenHandler {
	return &TokenHandler{tokenUsecase: tokenUsecase, userUsecase: userUsecase}
}

func (h *TokenHandler) Award(c echo.Context) error {
	user, err := resolveCurrentUser(c, h.userUsecase, "token")
	if err != nil {
		return err
	}

	balance, err := h.tokenUsecase.Award(c.Request().Context(), user)
	if err != nil {
		if errors.Is(err, domain.ErrQuestionBudgetExceeded) {
			return echo.NewHTTPError(http.StatusTooManyRequests, "ad view limit reached")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.ToTokenBalanceResponse(balance))
}

func (h *TokenHandler) Balance(c echo.Context) error {
	user, err := resolveCurrentUser(c, h.userUsecase, "token")
	if err != nil {
		return err
	}

	balance, err := h.tokenUsecase.Balance(c.Request().Context(), user)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.ToTokenBalanceResponse(balance))
}
