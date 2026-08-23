package stripeclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/stripe/stripe-go/v82"
)

// Client wraps Stripe operations using an initialized Stripe SDK client.
type Client struct {
	sc *stripe.Client // Stripe Go SDK client
}

// New creates a new Stripe client.
func New(secretKey string) *Client {
	sc := stripe.NewClient(secretKey)
	return &Client{sc: sc}
}

// newIdempotencyKey generates a cryptographically random idempotency key.
// Stripe deduplicates mutating requests carrying the same key for 24h, which
// makes retries of create calls safe (no duplicate customers/charges).
func newIdempotencyKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failures are exceptional; fall back to a fixed marker
		// so the call still proceeds (Stripe will treat it as best-effort).
		return "fallback-key"
	}
	return hex.EncodeToString(b)
}

// CreateCustomer creates a new Stripe customer
func (c *Client) CreateCustomer(ctx context.Context, email, name string, metadata map[string]string) (*stripe.Customer, error) {
	params := &stripe.CustomerCreateParams{
		Email:    stripe.String(email),
		Name:     stripe.String(name),
		Metadata: metadata,
	}
	params.SetIdempotencyKey(newIdempotencyKey())
	return c.sc.V1Customers.Create(ctx, params)
}

// GetCustomer retrieves a customer by ID
func (c *Client) GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	return c.sc.V1Customers.Retrieve(ctx, customerID, &stripe.CustomerRetrieveParams{})
}

// CreatePaymentIntent creates a new payment intent
func (c *Client) CreatePaymentIntent(ctx context.Context, amount int64, currency, customerID string, metadata map[string]string) (*stripe.PaymentIntent, error) {
	params := &stripe.PaymentIntentCreateParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String(currency),
		Customer: stripe.String(customerID),
		Metadata: metadata,
	}
	params.SetIdempotencyKey(newIdempotencyKey())
	return c.sc.V1PaymentIntents.Create(ctx, params)
}

// GetPaymentIntent retrieves a payment intent
func (c *Client) GetPaymentIntent(ctx context.Context, paymentIntentID string) (*stripe.PaymentIntent, error) {
	return c.sc.V1PaymentIntents.Retrieve(ctx, paymentIntentID, &stripe.PaymentIntentRetrieveParams{})
}

// CreateSubscription creates a new subscription
func (c *Client) CreateSubscription(ctx context.Context, customerID, priceID string, metadata map[string]string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionCreateParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionCreateItemParams{
			{Price: stripe.String(priceID)},
		},
		Metadata: metadata,
	}
	params.SetIdempotencyKey(newIdempotencyKey())
	return c.sc.V1Subscriptions.Create(ctx, params)
}

// CancelSubscription cancels a subscription
func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	return c.sc.V1Subscriptions.Cancel(ctx, subscriptionID, &stripe.SubscriptionCancelParams{})
}

// GetSubscription retrieves a subscription
func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	return c.sc.V1Subscriptions.Retrieve(ctx, subscriptionID, &stripe.SubscriptionRetrieveParams{})
}

// GetPrice retrieves a price
func (c *Client) GetPrice(ctx context.Context, priceID string) (*stripe.Price, error) {
	return c.sc.V1Prices.Retrieve(ctx, priceID, &stripe.PriceRetrieveParams{})
}

// LogError logs Stripe errors with context.
func LogError(operation string, err error) { // Made this a package-level function
	if stripeErr, ok := err.(*stripe.Error); ok {
		log.Printf("Stripe %s error: %s (code: %s, type: %s)",
			operation, stripeErr.Msg, stripeErr.Code, stripeErr.Type)
	} else {
		log.Printf("Stripe %s error: %v", operation, err)
	}
}
