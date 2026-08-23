package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"auth-service/internal/config"
	"auth-service/internal/service"
)

type contextKey string

// userIDContextKey is the typed context key under which AuthMiddleware stores
// the authenticated user id. (Previously a bare string key was used, which
// risks collisions across packages.)
const userIDContextKey contextKey = "userID"

type Handler struct {
	service service.Service
	jwtSvc  JWTService
	log     *zerolog.Logger
}

func New(svc service.Service, log *zerolog.Logger, jwtCfg config.JWTConfig) *Handler {
	jwtSvc := NewJWTService(jwtCfg.SecretKey, jwtCfg.AccessTokenTTL, jwtCfg.RefreshTokenTTL)
	return &Handler{
		service: svc,
		jwtSvc:  jwtSvc,
		log:     log,
	}
}

// RegisterRequest represents the request body for user registration
type RegisterRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// LoginRequest represents the request body for user login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshTokenRequest represents the request body for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// UpdateUserRequest represents the request body for updating user information
type UpdateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// ChangePasswordRequest represents the request body for changing a password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// --- Lightweight request validation (dependency-free) ---

func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	at := strings.Index(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	dot := strings.LastIndex(email[at+1:], ".")
	return dot > 0 && dot < len(email[at+1:])-1
}

func (req RegisterRequest) validate() error {
	if !validEmail(req.Email) {
		return errors.New("invalid email")
	}
	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		return errors.New("first_name and last_name are required")
	}
	return nil
}

func (req LoginRequest) validate() error {
	if !validEmail(req.Email) {
		return errors.New("invalid email")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func (req UpdateUserRequest) validate() error {
	if strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		return errors.New("first_name and last_name are required")
	}
	return nil
}

func (req ChangePasswordRequest) validate() error {
	if req.CurrentPassword == "" {
		return errors.New("current_password is required")
	}
	if len(req.NewPassword) < 8 {
		return errors.New("new_password must be at least 8 characters")
	}
	return nil
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.service.HealthCheck(); err != nil {
		h.log.Error().Err(err).Msg("Health check failed")
		h.respondWithError(w, http.StatusServiceUnavailable, "Service Unavailable")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Register handles user registration
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := req.validate(); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.Register(r.Context(), req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		h.log.Error().Err(err).Msg("Registration failed")
		statusCode := http.StatusInternalServerError
		if err.Error() == "user with this email already exists" {
			statusCode = http.StatusConflict
		}
		h.respondWithError(w, statusCode, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusCreated, resp)
}

// Login handles user login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := req.validate(); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.log.Error().Err(err).Msg("Login failed")
		statusCode := http.StatusInternalServerError
		if err.Error() == "invalid email or password" {
			statusCode = http.StatusUnauthorized
		}
		h.respondWithError(w, statusCode, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusOK, resp)
}

// RefreshToken handles token refresh
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		h.respondWithError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	tokens, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.log.Error().Err(err).Msg("Token refresh failed")
		statusCode := http.StatusInternalServerError
		if err.Error() == "invalid refresh token" || err.Error() == "user not found" {
			statusCode = http.StatusUnauthorized
		}
		h.respondWithError(w, statusCode, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusOK, tokens)
}

// VerifyToken validates the bearer token on the request and echoes the
// claims. Useful for other services and smoke tests.
func (h *Handler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "Invalid authorization header format")
		return
	}

	claims, err := h.jwtSvc.ValidateToken(token, AccessToken)
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "Invalid or expired token")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"user_id": claims.UserID,
		"email":   claims.Email,
		"type":    claims.Type,
	})
}

// GetCurrentUser gets the current authenticated user
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.service.GetUserByID(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to get user")
		h.respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	h.respondWithJSON(w, http.StatusOK, user)
}

// UpdateUser updates the current user's information
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := req.validate(); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateUser(r.Context(), userID, req.FirstName, req.LastName); err != nil {
		h.log.Error().Err(err).Msg("Failed to update user")
		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}
		h.respondWithError(w, statusCode, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "User updated successfully"})
}

// ChangePassword changes the current user's password
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := req.validate(); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		h.log.Error().Err(err).Msg("Failed to change password")
		statusCode := http.StatusInternalServerError
		switch err.Error() {
		case "user not found":
			statusCode = http.StatusNotFound
		case "current password is incorrect":
			statusCode = http.StatusUnauthorized
		}
		h.respondWithError(w, statusCode, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}

// bearerToken extracts a Bearer token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || !strings.EqualFold(tokenParts[0], "bearer") {
		return "", false
	}
	return tokenParts[1], true
}

// userIDFromContext reads the authenticated user id set by AuthMiddleware.
func userIDFromContext(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(userIDContextKey).(string)
	return userID, ok && userID != ""
}

// AuthMiddleware is a middleware to authenticate requests
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			h.respondWithError(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		claims, err := h.jwtSvc.ValidateToken(token, AccessToken)
		if err != nil {
			h.respondWithError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// respondWithJSON sends a JSON response with the given status code and data
func (h *Handler) respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.log.Error().Err(err).Msg("Failed to encode response")
		}
	}
}

// respondWithError sends a JSON error response with the given status code and message
func (h *Handler) respondWithError(w http.ResponseWriter, statusCode int, message string) {
	h.respondWithJSON(w, statusCode, ErrorResponse{Error: message})
}

// JWTService is an interface for JWT operations
type JWTService interface {
	GenerateTokenPair(userID, email string) (accessToken, refreshToken string, err error)
	ValidateToken(tokenString string, expectedType TokenType) (*Claims, error)
}

// jwtService is the HS256 implementation of JWTService.
type jwtService struct {
	secretKey       string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService(secretKey string, accessTokenTTL, refreshTokenTTL int) JWTService {
	return &jwtService{
		secretKey:       secretKey,
		accessTokenTTL:  time.Duration(accessTokenTTL) * time.Minute,
		refreshTokenTTL: time.Duration(refreshTokenTTL) * time.Minute,
	}
}

// GenerateTokenPair generates a new access and refresh token pair
func (s *jwtService) GenerateTokenPair(userID, email string) (string, string, error) {
	accessToken, err := s.generateToken(userID, email, AccessToken)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := s.generateToken(userID, email, RefreshToken)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token and returns its claims
func (s *jwtService) ValidateToken(tokenString string, expectedType TokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.secretKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if claims.Type != expectedType {
			return nil, errors.New("invalid token type")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// generateToken generates a new JWT token
func (s *jwtService) generateToken(userID, email string, tokenType TokenType) (string, error) {
	var ttl time.Duration
	if tokenType == AccessToken {
		ttl = s.accessTokenTTL
	} else {
		ttl = s.refreshTokenTTL
	}

	claims := &Claims{
		UserID: userID,
		Email:  email,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secretKey))
}

// TokenType represents the type of JWT token
type TokenType string

const (
	// AccessToken is a short-lived token for API access
	AccessToken TokenType = "access"
	// RefreshToken is a long-lived token used to refresh access tokens
	RefreshToken TokenType = "refresh"
)

// Claims represents the JWT claims
type Claims struct {
	UserID string    `json:"user_id"`
	Email  string    `json:"email"`
	Type   TokenType `json:"type"`
	jwt.RegisteredClaims
}

// Valid validates the claims
func (c *Claims) Valid() error {
	if c.Type != AccessToken && c.Type != RefreshToken {
		return errors.New("invalid token type")
	}
	return c.RegisteredClaims.Valid()
}
