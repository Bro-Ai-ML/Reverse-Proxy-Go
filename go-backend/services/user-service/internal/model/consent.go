package model

// ConsentUpdate représente les préférences de consentement utilisateur pour la conformité RGPD
type ConsentUpdate struct {
	// MarketingConsent indicates if the user consents to marketing communications
	MarketingConsent bool `json:"marketing_consent"`

	// DataPortability indicates if the user allows data portability
	DataPortability bool `json:"data_portability"`

	// TermsAccepted indicates if the user has accepted the latest terms and conditions
	TermsAccepted bool `json:"terms_accepted"`

	// TermsVersion specifies the version of the terms that were accepted
	TermsVersion string `json:"terms_version"`
}

// ConsentResponse représente le statut de consentement actuel d'un utilisateur
type ConsentResponse struct {
	UserID           string `json:"user_id"`
	Email            string `json:"email"`
	MarketingConsent bool   `json:"marketing_consent"`
	DataPortability  bool   `json:"data_portability"`
	TermsAccepted    bool   `json:"terms_accepted"`
	TermsVersion     string `json:"terms_version"`
	LastUpdated      string `json:"last_updated,omitempty"`
}
