package contracts

import "time"

// Usage represents API usage (tokens, calls, etc.)
type Usage struct {
	ID         string            `json:"id"`
	CustomerID string            `json:"customer_id"`
	ProductID  string            `json:"product_id"` // "gpt-4", "claude-3", etc.
	Quantity   int64             `json:"quantity"`   // tokens, calls, etc.
	UnitType   string            `json:"unit_type"`  // "tokens", "requests", "minutes"
	UnitPrice  int64             `json:"unit_price"` // price per unit in cents
	TotalCost  int64             `json:"total_cost"` // total cost in cents
	Metadata   map[string]string `json:"metadata"`
	Timestamp  time.Time         `json:"timestamp"`
	CreatedAt  time.Time         `json:"created_at"`
}

// UsageRecord for Stripe metered billing
type UsageRecord struct {
	CustomerID     string `json:"customer_id"`
	SubscriptionID string `json:"subscription_id"`
	PriceID        string `json:"price_id"`
	Quantity       int64  `json:"quantity"`
	Action         string `json:"action"` // "increment", "set"
	Timestamp      int64  `json:"timestamp"`
}

// BillingTier represents different service tiers
type BillingTier struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`           // "Free", "Pro", "Enterprise"
	MonthlyPrice  int64    `json:"monthly_price"`  // base subscription price
	IncludedUnits int64    `json:"included_units"` // free tokens/calls per month
	OverageRate   int64    `json:"overage_rate"`   // price per unit over limit
	RateLimit     int64    `json:"rate_limit"`     // requests per minute
	Features      []string `json:"features"`       // ["priority_support", "custom_models"]
}

// CustomerBilling represents customer's billing state
type CustomerBilling struct {
	CustomerID       string    `json:"customer_id"`
	TierID           string    `json:"tier_id"`
	SubscriptionID   string    `json:"subscription_id"`
	CurrentUsage     int64     `json:"current_usage"` // this month's usage
	UsageLimit       int64     `json:"usage_limit"`   // monthly limit
	OverageUsage     int64     `json:"overage_usage"` // usage over limit
	BillingPeriodEnd time.Time `json:"billing_period_end"`
	Status           string    `json:"status"` // "active", "suspended", "over_limit"
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ApiKey represents customer API keys with billing tracking
type ApiKey struct {
	ID          string            `json:"id"`
	CustomerID  string            `json:"customer_id"`
	KeyHash     string            `json:"key_hash"`    // hashed API key
	Name        string            `json:"name"`        // user-defined name
	Permissions []string          `json:"permissions"` // ["read", "write", "admin"]
	RateLimit   int64             `json:"rate_limit"`  // requests per minute
	IsActive    bool              `json:"is_active"`
	LastUsed    *time.Time        `json:"last_used"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at"`
}

// UsageEvent for real-time usage tracking
type UsageEvent struct {
	CustomerID         string            `json:"customer_id"`
	ApiKeyID           string            `json:"api_key_id"`
	ProductID          string            `json:"product_id"`
	SubscriptionItemID string            `json:"subscription_item_id"`
	Endpoint           string            `json:"endpoint"`
	Quantity           int64             `json:"quantity"`
	UnitType           string            `json:"unit_type"`
	Metadata           map[string]string `json:"metadata"`
	Timestamp          time.Time         `json:"timestamp"`
}
