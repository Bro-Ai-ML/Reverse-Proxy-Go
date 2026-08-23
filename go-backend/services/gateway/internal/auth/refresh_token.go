package auth

import (
	"sync"
	"time"
)

type RefreshToken struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Roles      []string  `json:"roles"` // roles are persisted so a refresh never downgrades privileges
	TokenHash  string    `json:"-"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Revoked    bool      `json:"revoked"`
	Token      string    `json:"token,omitempty"` // only populated in responses, never stored
}

type RefreshTokenStore interface {
	Create(token *RefreshToken) error
	GetByID(id string) (*RefreshToken, error)
	// FindByTokenHash returns the live (non-revoked, non-expired) token whose
	// hash matches. Implementations must compare hashes, never raw tokens.
	FindByTokenHash(hash string) (*RefreshToken, error)
	// IsConsumed reports whether the hash belongs to a token that was already
	// rotated away. Presenting such a token indicates theft and triggers
	// revocation of all of the user's sessions.
	IsConsumed(hash string) bool
	// MarkConsumed records that a token hash has been rotated.
	MarkConsumed(hash string)
	Revoke(id string) error
	RevokeAllForUser(userID string) error
	DeleteExpired() error
}

// consumedRetention bounds how long rotated-token hashes are kept for reuse
// detection; it must be >= the refresh token lifetime.
const consumedRetention = 14 * 24 * time.Hour

type InMemoryRefreshTokenStore struct {
	mu       sync.RWMutex
	tokens   map[string]*RefreshToken
	consumed map[string]time.Time // hash -> time it was rotated (reuse detection)
}

func NewInMemoryRefreshTokenStore() *InMemoryRefreshTokenStore {
	return &InMemoryRefreshTokenStore{
		tokens:   make(map[string]*RefreshToken),
		consumed: make(map[string]time.Time),
	}
}

func (s *InMemoryRefreshTokenStore) Create(token *RefreshToken) error {
	stored := *token
	stored.Token = "" // never keep the raw token in memory, only its hash
	s.mu.Lock()
	s.tokens[stored.ID] = &stored
	s.mu.Unlock()
	return nil
}

func (s *InMemoryRefreshTokenStore) GetByID(id string) (*RefreshToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, exists := s.tokens[id]
	if !exists || token.Revoked || token.ExpiresAt.Before(time.Now()) {
		return nil, ErrInvalidToken
	}
	return token, nil
}

func (s *InMemoryRefreshTokenStore) FindByTokenHash(hash string) (*RefreshToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tokens {
		if t.TokenHash == hash && !t.Revoked && t.ExpiresAt.After(time.Now()) {
			return t, nil
		}
	}
	return nil, ErrInvalidToken
}

func (s *InMemoryRefreshTokenStore) IsConsumed(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.consumed[hash]
	return ok
}

func (s *InMemoryRefreshTokenStore) MarkConsumed(hash string) {
	s.mu.Lock()
	s.consumed[hash] = time.Now()
	s.mu.Unlock()
}

func (s *InMemoryRefreshTokenStore) Revoke(id string) error {
	s.mu.Lock()
	if token, exists := s.tokens[id]; exists {
		token.Revoked = true
	}
	s.mu.Unlock()
	return nil
}

func (s *InMemoryRefreshTokenStore) RevokeAllForUser(userID string) error {
	s.mu.Lock()
	for _, token := range s.tokens {
		if token.UserID == userID {
			token.Revoked = true
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *InMemoryRefreshTokenStore) DeleteExpired() error {
	now := time.Now()
	s.mu.Lock()
	for id, token := range s.tokens {
		if token.ExpiresAt.Before(now) {
			delete(s.tokens, id)
		}
	}
	for hash, consumedAt := range s.consumed {
		if now.Sub(consumedAt) > consumedRetention {
			delete(s.consumed, hash)
		}
	}
	s.mu.Unlock()
	return nil
}
