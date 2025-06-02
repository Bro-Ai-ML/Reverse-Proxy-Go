package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (m *JWTManager) RefreshHandler(store RefreshTokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		if req.RefreshToken == "" {
			respondWithError(w, http.StatusBadRequest, "Refresh token is required")
			return
		}
		// Recherche du token par valeur (hash)
		token, err := getByToken(store, req.RefreshToken)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
			return
		}
		if time.Now().After(token.ExpiresAt) {
			store.Revoke(token.ID)
			respondWithError(w, http.StatusUnauthorized, "Refresh token expired")
			return
		}
		if err := store.Revoke(token.ID); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to revoke token")
			return
		}
		roles := []string{"user"} // À remplacer par la logique réelle
		accessToken, err := m.GenerateToken(token.UserID, roles, 15*time.Minute)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate access token")
			return
		}
		newRefreshToken, err := generateRefreshToken(store, token.UserID, 7*24*time.Hour)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate refresh token")
			return
		}
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"access_token":  accessToken,
			"refresh_token": newRefreshToken.Token,
			"expires_in":    int64((15 * time.Minute).Seconds()),
			"token_type":    "Bearer",
		})
	}
}

// getByToken recherche un refresh token par sa valeur (hash)
func getByToken(store RefreshTokenStore, token string) (*RefreshToken, error) {
	hash := hashToken(token)
	if s, ok := store.(*InMemoryRefreshTokenStore); ok {
		for _, t := range s.tokens {
			if t.TokenHash == hash && !t.Revoked && t.ExpiresAt.After(time.Now()) {
				return t, nil
			}
		}
	}
	return nil, ErrInvalidToken
}
