package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func (m *JWTManager) LoginHandler(store RefreshTokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		if req.Username == "" || req.Password == "" {
			respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		userID, roles, err := validateCredentials(req.Username, req.Password)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		accessToken, err := m.GenerateToken(userID, roles, 15*time.Minute)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
			return
		}
		refreshToken, err := generateRefreshToken(store, userID, 7*24*time.Hour)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate refresh token")
			return
		}
		respondWithJSON(w, http.StatusOK, TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken.Token,
			ExpiresIn:    int64((15 * time.Minute).Seconds()),
			TokenType:    "Bearer",
		})
	}
}

func validateCredentials(username, password string) (string, []string, error) {
	if username == "user" && password == "pass" {
		return "user123", []string{"user"}, nil
	}
	return "", nil, ErrInvalidToken
}

func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func LogoutHandler(store RefreshTokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		token, err := getByToken(store, req.RefreshToken)
		if err != nil {
			RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
			return
		}
		if err := store.Revoke(token.ID); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to revoke token")
			return
		}
		RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
	}
}
