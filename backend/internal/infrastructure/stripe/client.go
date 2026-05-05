package stripe

import (
	"context"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	stripeapi "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
)

type CheckoutClient struct {
	secretKey  string
	priceID    string
	successURL string
	cancelURL  string
}

func NewCheckoutClientFromEnv() *CheckoutClient {
	return &CheckoutClient{
		secretKey:  strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")),
		priceID:    strings.TrimSpace(os.Getenv("STRIPE_PRICE_ID_MONTHLY")),
		successURL: strings.TrimSpace(os.Getenv("STRIPE_SUCCESS_URL")),
		cancelURL:  strings.TrimSpace(os.Getenv("STRIPE_CANCEL_URL")),
	}
}

func (c *CheckoutClient) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, email string) (string, error) {
	if c.secretKey == "" || c.priceID == "" || c.successURL == "" || c.cancelURL == "" {
		return "", domain.ErrInvalidInput
	}

	stripeapi.Key = c.secretKey
	params := &stripeapi.CheckoutSessionParams{
		Mode:              stripeapi.String(string(stripeapi.CheckoutSessionModeSubscription)),
		SuccessURL:        stripeapi.String(c.successURL),
		CancelURL:         stripeapi.String(c.cancelURL),
		ClientReferenceID: stripeapi.String(userID.String()),
		LineItems: []*stripeapi.CheckoutSessionLineItemParams{{
			Price:    stripeapi.String(c.priceID),
			Quantity: stripeapi.Int64(1),
		}},
		Metadata: map[string]string{
			"user_id": userID.String(),
		},
	}
	if normalizedEmail := strings.TrimSpace(email); normalizedEmail != "" {
		params.CustomerEmail = stripeapi.String(normalizedEmail)
	}
	params.Context = ctx

	result, err := session.New(params)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
