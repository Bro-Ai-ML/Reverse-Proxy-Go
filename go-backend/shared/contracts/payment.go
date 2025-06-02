package contracts

import "time"

// Payment represents a payment transaction
type Payment struct {
	ID          string            `json:"id"`
	CustomerID  string            `json:"customer_id"`
	Amount      int64             `json:"amount"` // in cents
	Currency    string            `json:"currency"`
	Status      string            `json:"status"`
	StripeID    string            `json:"stripe_id"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Customer represents a customer
type Customer struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	StripeID  string            `json:"stripe_id"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Subscription represents a subscription
type Subscription struct {
	ID               string    `json:"id"`
	CustomerID       string    `json:"customer_id"`
	PriceID          string    `json:"price_id"`
	Status           string    `json:"status"`
	StripeID         string    `json:"stripe_id"`
	CurrentPeriodEnd time.Time `json:"current_period_end"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PaymentRequest represents a payment creation request
type PaymentRequest struct {
	CustomerID  string            `json:"customer_id"`
	Amount      int64             `json:"amount"`
	Currency    string            `json:"currency"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

// PaymentResponse represents a payment response
type PaymentResponse struct {
	Payment      *Payment `json:"payment"`
	ClientSecret string   `json:"client_secret,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
