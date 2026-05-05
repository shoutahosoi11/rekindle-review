package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type billingRepository struct {
	db *sql.DB
}

func NewBillingRepository(db *sql.DB) domain.BillingRepository {
	return &billingRepository{db: db}
}

func (r *billingRepository) MarkCheckoutCompleted(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, expiresAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET plan = 'premium',
    stripe_customer_id = NULLIF($2, ''),
    stripe_subscription_id = NULLIF($3, ''),
    subscription_expires_at = $4,
    updated_at = NOW()
WHERE id = $1
`, userID, strings.TrimSpace(customerID), strings.TrimSpace(subscriptionID), expiresAt)
	if err != nil {
		return fmt.Errorf("billing repo: mark checkout completed: %w", err)
	}
	return nil
}

func (r *billingRepository) UpdateSubscription(ctx context.Context, customerID, subscriptionID string, expiresAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET plan = 'premium',
    stripe_subscription_id = NULLIF($2, ''),
    subscription_expires_at = $3,
    updated_at = NOW()
WHERE stripe_customer_id = $1
   OR stripe_subscription_id = $2
`, strings.TrimSpace(customerID), strings.TrimSpace(subscriptionID), expiresAt)
	if err != nil {
		return fmt.Errorf("billing repo: update subscription: %w", err)
	}
	return nil
}

func (r *billingRepository) CancelSubscription(ctx context.Context, customerID, subscriptionID string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET plan = 'free',
    subscription_expires_at = NULL,
    updated_at = NOW()
WHERE stripe_customer_id = $1
   OR stripe_subscription_id = $2
`, strings.TrimSpace(customerID), strings.TrimSpace(subscriptionID))
	if err != nil {
		return fmt.Errorf("billing repo: cancel subscription: %w", err)
	}
	return nil
}
