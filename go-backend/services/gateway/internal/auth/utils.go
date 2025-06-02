package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateRefreshToken(store RefreshTokenStore, userID string, duration time.Duration) (*RefreshToken, error) {
	token, tokenHash, err := generateToken()
	if err != nil {
		return nil, err
	}
	rt := &RefreshToken{
		ID:         generateID(),
		UserID:     userID,
		Token:      token,
		TokenHash:  tokenHash,
		ExpiresAt:  time.Now().Add(duration),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		Revoked:    false,
	}
	if err := store.Create(rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func generateToken() (string, string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", err
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)
	tokenHash := hashToken(token)
	return token, tokenHash, nil
}
