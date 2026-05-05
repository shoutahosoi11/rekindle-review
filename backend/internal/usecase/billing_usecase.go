package usecase

import (
	"context"
	"log"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type BillingUsecase struct {
	sessionCreator domain.CheckoutSessionCreator
	webhook        domain.StripeWebhookValidator
	billingRepo    domain.BillingRepository
}

func NewBillingUsecase(sessionCreator domain.CheckoutSessionCreator, webhook domain.StripeWebhookValidator, billingRepo domain.BillingRepository) *BillingUsecase {
	return &BillingUsecase{sessionCreator: sessionCreator, webhook: webhook, billingRepo: billingRepo}
}

func (u *BillingUsecase) CreateCheckoutSession(ctx context.Context, user *domain.User, email string) (string, error) {
	return u.sessionCreator.CreateCheckoutSession(ctx, user.ID, email)
}

func (u *BillingUsecase) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	event, err := u.webhook.ConstructEvent(payload, signature)
	if err != nil {
		return err
	}

	switch event.Type {
	case "checkout.session.completed":
		return u.billingRepo.MarkCheckoutCompleted(ctx, event.UserID, event.CustomerID, event.SubscriptionID, event.ExpiresAt)
	case "customer.subscription.updated":
		return u.billingRepo.UpdateSubscription(ctx, event.CustomerID, event.SubscriptionID, event.ExpiresAt)
	case "customer.subscription.deleted":
		return u.billingRepo.CancelSubscription(ctx, event.CustomerID, event.SubscriptionID)
	case "invoice.payment_failed":
		log.Printf("stripe webhook: invoice payment failed customer_id=%s subscription_id=%s", event.CustomerID, event.SubscriptionID)
	}

	return nil
}
