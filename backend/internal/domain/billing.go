package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CheckoutSessionCreator interface {
	CreateCheckoutSession(ctx context.Context, userID uuid.UUID, email string) (string, error)
}

type StripeWebhookValidator interface {
	ConstructEvent(payload []byte, signatureHeader string) (StripeWebhookEvent, error)
}

type StripeWebhookEvent struct {
	Type           string
	CustomerID     string
	SubscriptionID string
	UserID         uuid.UUID
	ExpiresAt      *time.Time
}

type BillingRepository interface {
	MarkCheckoutCompleted(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, expiresAt *time.Time) error
	UpdateSubscription(ctx context.Context, customerID, subscriptionID string, expiresAt *time.Time) error
	CancelSubscription(ctx context.Context, customerID, subscriptionID string) error
}
