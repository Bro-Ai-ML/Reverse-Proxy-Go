package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"auth-service/internal/config"
	"auth-service/internal/service"
)

type Handler struct {
	service service.Service
	jwtSvc  *JWTService
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
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
}

// LoginRequest represents the request body for user login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshTokenRequest represents the request body for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// UpdateUserRequest represents the request body for updating user information
type UpdateUserRequest struct {
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
}

// ChangePasswordRequest represents the request body for changing password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error string `json:"error"`
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

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
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

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
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

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
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

// GetCurrentUser gets the current authenticated user
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)

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
	userID := r.Context().Value("userID").(string)

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
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
	userID := r.Context().Value("userID").(string)

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
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

// AuthMiddleware is a middleware to authenticate requests
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.respondWithError(w, http.StatusUnauthorized, "Authorization header is required")
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
			h.respondWithError(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		token := tokenParts[1]
		claims, err := h.jwtSvc.ValidateToken(token, AccessToken)
		if err != nil {
			h.respondWithError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Add user ID to context
		ctx := context.WithValue(r.Context(), "userID", claims.UserID)
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

// JWTService implementation
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
	})

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
	// RefreshToken is a long-lived token for refreshing access tokens
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
