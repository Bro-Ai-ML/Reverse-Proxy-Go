package model

import "testing"

func TestConsentStructs(t *testing.T) {
	c := ConsentUpdate{MarketingConsent: true, DataPortability: false, TermsAccepted: true, TermsVersion: "v1"}
	if !c.MarketingConsent {
		t.Error("expected MarketingConsent true")
	}
	resp := ConsentResponse{
		UserID:           "id",
		Email:            "test@example.com",
		MarketingConsent: true,
		DataPortability:  false,
		TermsAccepted:    true,
		TermsVersion:     "v1",
		LastUpdated:      "2024-01-01",
	}
	if resp.UserID != "id" || !resp.MarketingConsent || resp.TermsVersion != "v1" {
		t.Error("ConsentResponse fields not set correctly")
	}
}
