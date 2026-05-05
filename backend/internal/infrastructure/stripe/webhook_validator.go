package stripe

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	stripeapi "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

type WebhookValidator struct {
	secret string
}

func NewWebhookValidatorFromEnv() *WebhookValidator {
	return &WebhookValidator{secret: strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))}
}

func (v *WebhookValidator) ConstructEvent(payload []byte, signatureHeader string) (domain.StripeWebhookEvent, error) {
	if v.secret == "" {
		return domain.StripeWebhookEvent{}, domain.ErrInvalidInput
	}

	event, err := webhook.ConstructEvent(payload, signatureHeader, v.secret)
	if err != nil {
		return domain.StripeWebhookEvent{}, err
	}

	result := domain.StripeWebhookEvent{Type: string(event.Type)}
	switch event.Type {
	case "checkout.session.completed":
		var session stripeapi.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return domain.StripeWebhookEvent{}, err
		}
		userID, err := checkoutUserID(session)
		if err != nil {
			return domain.StripeWebhookEvent{}, err
		}
		result.UserID = userID
		if session.Customer != nil {
			result.CustomerID = session.Customer.ID
		}
		if session.Subscription != nil {
			result.SubscriptionID = session.Subscription.ID
		}
	case "customer.subscription.updated", "customer.subscription.deleted":
		var subscription stripeapi.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			return domain.StripeWebhookEvent{}, err
		}
		if subscription.Customer != nil {
			result.CustomerID = subscription.Customer.ID
		}
		result.SubscriptionID = subscription.ID
		if subscription.CurrentPeriodEnd > 0 {
			expiresAt := time.Unix(subscription.CurrentPeriodEnd, 0)
			result.ExpiresAt = &expiresAt
		}
	case "invoice.payment_failed":
		var invoice stripeapi.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return domain.StripeWebhookEvent{}, err
		}
		if invoice.Customer != nil {
			result.CustomerID = invoice.Customer.ID
		}
		if invoice.Subscription != nil {
			result.SubscriptionID = invoice.Subscription.ID
		}
	}

	return result, nil
}

func checkoutUserID(session stripeapi.CheckoutSession) (uuid.UUID, error) {
	if userID, err := uuid.Parse(strings.TrimSpace(session.ClientReferenceID)); err == nil {
		return userID, nil
	}
	if rawID := strings.TrimSpace(session.Metadata["user_id"]); rawID != "" {
		return uuid.Parse(rawID)
	}
	return uuid.Nil, domain.ErrInvalidInput
}
