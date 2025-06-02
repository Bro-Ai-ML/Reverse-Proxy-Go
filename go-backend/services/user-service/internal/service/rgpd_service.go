package service

import (
	"context"
	"encoding/json"

	"stripe-demo/services/user-service/internal/model"
	"stripe-demo/services/user-service/internal/repository"

	"github.com/rs/zerolog/log"
)

type RGPDServices interface {
	// ExportUserData exports all personal data for a user in JSON format
	ExportUserData(ctx context.Context, userID string) ([]byte, error)

	// UpdateConsent updates user consent preferences
	UpdateConsent(ctx context.Context, userID string, consent *model.ConsentUpdate) error

	// AnonymizeUser removes all personal data for a user
	AnonymizeUser(ctx context.Context, userID string) error
}

type RGPDSvc struct {
	repo repository.UserRepository
}

func NewRGPDSvc(repo repository.UserRepository) *RGPDSvc {
	return &RGPDSvc{
		repo: repo,
	}
}

func (s *RGPDSvc) ExportUserData(ctx context.Context, userID string) ([]byte, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get all data related to the user
	data := user.RGPDData()

	// Add any additional data from other services here
	// Example: data["activities"] = s.activityService.GetUserActivities(userID)

	// Convert to JSON with proper indentation
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to marshal user data")
		return nil, err
	}

	return jsonData, nil
}

func (s *RGPDSvc) UpdateConsent(ctx context.Context, userID string, consent *model.ConsentUpdate) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Update consent fields
	user.MarketingConsent = consent.MarketingConsent
	user.DataPortability = consent.DataPortability
	user.TermsAccepted = consent.TermsAccepted
	user.TermsVersion = consent.TermsVersion

	// Update the user in the database
	return s.repo.UpdateUser(ctx, user)
}

func (s *RGPDSvc) AnonymizeUser(ctx context.Context, userID string) error {
	// Get the user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Anonymize the user
	user.Anonymize()

	// Save the anonymized user
	return s.repo.UpdateUser(ctx, user)
}
