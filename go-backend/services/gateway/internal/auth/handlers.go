package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
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

// CredentialValidator authenticates a username/password pair and returns the
// user id and roles. The gateway never embeds credentials in code: the
// production implementation should query a user store (e.g. auth-service).
type CredentialValidator interface {
	Validate(username, password string) (userID string, roles []string, err error)
}

// EnvDemoCredentialValidator is an opt-in single-user validator for local
// demos. It is only active when GATEWAY_DEMO_USERNAME and
// GATEWAY_DEMO_PASSWORD are both set; otherwise every login fails closed.
// Passwords are compared in constant time.
type EnvDemoCredentialValidator struct {
	username string
	password string
}

func NewEnvDemoCredentialValidator() *EnvDemoCredentialValidator {
	return &EnvDemoCredentialValidator{
		username: os.Getenv("GATEWAY_DEMO_USERNAME"),
		password: os.Getenv("GATEWAY_DEMO_PASSWORD"),
	}
}

func (v *EnvDemoCredentialValidator) Validate(username, password string) (string, []string, error) {
	if v.username == "" || v.password == "" {
		return "", nil, ErrInvalidToken // fail closed: no demo account configured
	}
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(v.username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(v.password)) == 1
	if !userOK || !passOK {
		return "", nil, ErrInvalidToken
	}
	return "user123", []string{"user", "admin"}, nil
}

func (m *JWTManager) LoginHandler(store RefreshTokenStore, creds CredentialValidator, tokenTTL, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		var req LoginRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		if req.Username == "" || req.Password == "" {
			respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		userID, roles, err := creds.Validate(req.Username, req.Password)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		accessToken, err := m.GenerateToken(userID, roles, tokenTTL)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
			return
		}
		refreshToken, err := generateRefreshToken(store, userID, roles, refreshTTL)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to generate refresh token")
			return
		}
		respondWithJSON(w, http.StatusOK, TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken.Token,
			ExpiresIn:    int64(tokenTTL.Seconds()),
			TokenType:    "Bearer",
		})
	}
}

func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func LogoutHandler(store RefreshTokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		token, err := store.FindByTokenHash(hashToken(req.RefreshToken))
		if err != nil {
			// Idempotent logout: unknown/expired tokens still succeed.
			RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
			return
		}
		if err := store.Revoke(token.ID); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to revoke token")
			return
		}
		store.MarkConsumed(hashToken(req.RefreshToken))
		RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
	}
}
