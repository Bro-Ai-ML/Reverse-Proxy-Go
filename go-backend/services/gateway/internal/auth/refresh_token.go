package auth

import (
	"time"
)

type RefreshToken struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TokenHash  string    `json:"-"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Revoked    bool      `json:"revoked"`
	Token      string    `json:"token,omitempty"` // pour la réponse
}

type RefreshTokenStore interface {
	Create(token *RefreshToken) error
	GetByID(id string) (*RefreshToken, error)
	Revoke(id string) error
	RevokeAllForUser(userID string) error
	DeleteExpired() error
}

type InMemoryRefreshTokenStore struct {
	tokens map[string]*RefreshToken
}

func NewInMemoryRefreshTokenStore() *InMemoryRefreshTokenStore {
	return &InMemoryRefreshTokenStore{
		tokens: make(map[string]*RefreshToken),
	}
}

func (s *InMemoryRefreshTokenStore) Create(token *RefreshToken) error {
	s.tokens[token.ID] = token
	return nil
}

func (s *InMemoryRefreshTokenStore) GetByID(id string) (*RefreshToken, error) {
	token, exists := s.tokens[id]
	if !exists || token.Revoked || token.ExpiresAt.Before(time.Now()) {
		return nil, ErrInvalidToken
	}
	return token, nil
}

func (s *InMemoryRefreshTokenStore) Revoke(id string) error {
	if token, exists := s.tokens[id]; exists {
		token.Revoked = true
	}
	return nil
}

func (s *InMemoryRefreshTokenStore) RevokeAllForUser(userID string) error {
	for _, token := range s.tokens {
		if token.UserID == userID {
			token.Revoked = true
		}
	}
	return nil
}

func (s *InMemoryRefreshTokenStore) DeleteExpired() error {
	now := time.Now()
	for id, token := range s.tokens {
		if token.ExpiresAt.Before(now) {
			delete(s.tokens, id)
		}
	}
	return nil
}
