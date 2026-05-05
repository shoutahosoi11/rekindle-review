package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/middleware"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type BillingUsecase interface {
	CreateCheckoutSession(ctx context.Context, user *domain.User, email string) (string, error)
	HandleWebhook(ctx context.Context, payload []byte, signature string) error
}

type StripeHandler struct {
	billingUsecase BillingUsecase
	userUsecase    usecase.UserUsecaseInterface
}

func NewStripeHandler(billingUsecase BillingUsecase, userUsecase usecase.UserUsecaseInterface) *StripeHandler {
	return &StripeHandler{billingUsecase: billingUsecase, userUsecase: userUsecase}
}

func (h *StripeHandler) CreateCheckoutSession(c echo.Context) error {
	user, err := resolveCurrentUser(c, h.userUsecase, "stripe")
	if err != nil {
		return err
	}

	email := ""
	if token, ok := middleware.GetFirebaseToken(c); ok && token != nil {
		if claimEmail, ok := token.Claims["email"].(string); ok {
			email = strings.TrimSpace(claimEmail)
		}
	}

	sessionURL, err := h.billingUsecase.CreateCheckoutSession(c.Request().Context(), user, email)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "stripe is not configured")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.CheckoutSessionResponse{URL: sessionURL})
}

func (h *StripeHandler) HandleWebhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	if err := h.billingUsecase.HandleWebhook(c.Request().Context(), payload, c.Request().Header.Get("Stripe-Signature")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid webhook")
	}

	return c.NoContent(http.StatusOK)
}
