package internal

import (
	"context"

	"github.com/stripe/stripe-go/v82"
)

// PaymentProvider defines the interface for payment operations
type PaymentProvider interface {
	CreatePaymentIntent(ctx context.Context, amount int64, currency, customerID string, metadata map[string]string) (*stripe.PaymentIntent, error)
	GetPaymentIntent(ctx context.Context, paymentIntentID string) (*stripe.PaymentIntent, error)
}
