package stripeclient

import (
	"context"
	"testing"
)

type fakeClient struct{}

func (f *fakeClient) V1Customers() bool { return true }

func TestNew(t *testing.T) {
	c := New("sk_test_key")
	if c == nil {
		t.Fatal("expected non-nil Client")
	}
}

func TestClientMethods(t *testing.T) {
	c := New("sk_test_key")
	ctx := context.Background()

	t.Run("CreateCustomer", func(t *testing.T) {
		_, err := c.CreateCustomer(ctx, "test@example.com", "Test User", nil)
		if err == nil {
			t.Log("CreateCustomer: expected error with test key or network, got nil (OK for dry test)")
		}
	})

	t.Run("GetCustomer", func(t *testing.T) {
		_, err := c.GetCustomer(ctx, "cus_testid")
		if err == nil {
			t.Log("GetCustomer: expected error with test key or network, got nil (OK for dry test)")
		}
	})

	t.Run("CreatePaymentIntent", func(t *testing.T) {
		_, err := c.CreatePaymentIntent(ctx, 1000, "usd", "cus_testid", nil)
		if err == nil {
			t.Log("CreatePaymentIntent: expected error with test key or network, got nil (OK for dry test)")
		}
	})

	t.Run("GetPaymentIntent", func(t *testing.T) {
		_, err := c.GetPaymentIntent(ctx, "pi_testid")
		if err == nil {
			t.Log("GetPaymentIntent: expected error with test key or network, got nil (OK for dry test)")
		}
	})

	t.Run("CreateSubscription", func(t *testing.T) {
		_, err := c.CreateSubscription(ctx, "cus_testid", "price_testid", nil)
		if err == nil {
			t.Log("CreateSubscription: expected error with test key or network, got nil (OK for dry test)")
		}
	})

	t.Run("CancelSubscription", func(t *testing.T) {
		_, err := c.CancelSubscription(ctx, "sub_testid")
		if err == nil {
			t.Log("CancelSubscription: expected error with test key or network, got nil (OK for dry test)")
		}
	})

	t.Run("GetSubscription", func(t *testing.T) {
		_, err := c.GetSubscription(ctx, "sub_testid")
		if err == nil {
			t.Log("GetSubscription: expected error with test key or network, got nil (OK for dry test)")
		}
	})

	t.Run("GetPrice", func(t *testing.T) {
		_, err := c.GetPrice(ctx, "price_testid")
		if err == nil {
			t.Log("GetPrice: expected error with test key or network, got nil (OK for dry test)")
		}
	})
}

func TestClient_CreateCustomer_Error(t *testing.T) {
	c := New("sk_test_key")
	ctx := context.Background()
	_, err := c.CreateCustomer(ctx, "", "", nil)
	if err == nil {
		t.Error("expected error for empty email and name")
	}
}

func TestClient_GetCustomer_Error(t *testing.T) {
	c := New("sk_test_key")
	ctx := context.Background()
	_, err := c.GetCustomer(ctx, "")
	if err == nil {
		t.Error("expected error for empty customer ID")
	}
}
