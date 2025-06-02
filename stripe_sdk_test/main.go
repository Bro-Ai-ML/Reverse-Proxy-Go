package main

import (
	"context"
	"fmt"
	"os"

	"github.com/stripe/stripe-go/v82"
)

func main() {
	// It's good practice to set the key before making any calls,
	// or use per-request API keys. For this test, we'll set it globally.
	// Ensure you have STRIPE_SECRET_KEY environment variable set for this to run,
	// though for a compile test, the actual key value doesn't strictly matter.
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	if stripe.Key == "" {
		fmt.Println("Warning: STRIPE_SECRET_KEY environment variable not set. Using a placeholder.")
		stripe.Key = "sk_test_yourtestkey" // Placeholder
	}

	ctx := context.Background()

	params := &stripe.CustomerCreateParams{
		Email: stripe.String("test_sdk_standalone@example.com"),
		Name:  stripe.String("Standalone Test"),
	}

	// The crucial test: using the new client-based API with context
	sc := stripe.NewClient(stripe.Key)
	_, err := sc.V1Customers.Create(ctx, params)
	if err != nil {
		// For a real run, handle the error. For a compile test, this is enough.
		// We expect an error if the key is invalid or network issues, but not a compile error.
		fmt.Printf("Error creating customer (expected if key is placeholder or network issue): %v\n", err)
		// Check if it's a Stripe error, which would indicate API interaction
		if stripeErr, ok := err.(*stripe.Error); ok {
			fmt.Printf("Stripe error: Type=%s, Code=%s, Msg=%s\n", stripeErr.Type, stripeErr.Code, stripeErr.Msg)
		}
		return
	}

	fmt.Println("Successfully compiled and called customer.New with stripe.WithContext.")
	fmt.Println("If you saw an error message above, it was likely an API authentication/network error, not a compilation error.")
}
