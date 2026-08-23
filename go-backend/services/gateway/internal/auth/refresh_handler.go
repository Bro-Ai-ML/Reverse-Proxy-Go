package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (m *JWTManager) RefreshHandler(store RefreshTokenStore, tokenTTL, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		if req.RefreshToken == "" {
			respondWithError(w, http.StatusBadRequest, "Refresh token is required")
			return
		}

		hash := hashToken(req.RefreshToken)

		// Reuse detection: presenting a token that was already rotated means
		// it was likely stolen. Revoke every session of the affected user.
		if store.IsConsumed(hash) {
			if stolen, err := findByHashIncludingRevoked(store, hash); err == nil {
				_ = store.RevokeAllForUser(stolen.UserID)
			}
			respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
			return
		}

		token, err := store.FindByTokenHash(hash)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
			return
		}

		// Rotation: consume the old token and issue a fresh pair. Roles are
		// carried over from the stored token — they used to be reset to
		// ["user"], silently downgrading admins on every refresh.
		if err := store.Revoke(token.ID); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to revoke token")
			return
		}
		store.MarkConsumed(hash)

		roles := token.Roles
		if len(roles) == 0 {
			roles = []string{"user"}
		}
		accessToken, err := m.GenerateToken(token.UserID, roles, tokenTTL)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate access token")
			return
		}
		newRefreshToken, err := generateRefreshToken(store, token.UserID, roles, refreshTTL)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate refresh token")
			return
		}
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"access_token":  accessToken,
			"refresh_token": newRefreshToken.Token,
			"expires_in":    int64(tokenTTL.Seconds()),
			"token_type":    "Bearer",
		})
	}
}

// findByHashIncludingRevoked is a best-effort lookup used by reuse detection
// to identify the owner of a replayed token.
func findByHashIncludingRevoked(store RefreshTokenStore, hash string) (*RefreshToken, error) {
	if s, ok := store.(*InMemoryRefreshTokenStore); ok {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, t := range s.tokens {
			if t.TokenHash == hash {
				return t, nil
			}
		}
	}
	return nil, ErrInvalidToken
}
