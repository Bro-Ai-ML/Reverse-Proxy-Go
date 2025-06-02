package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

type Claims struct {
	UserID string    `json:"user_id"`
	Email  string    `json:"email"`
	Type   TokenType `json:"type"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secretKey       string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewJWTService(secretKey string, accessTokenTTL, refreshTokenTTL int) *JWTService {
	return &JWTService{
		secretKey:       secretKey,
		accessTokenTTL:  time.Duration(accessTokenTTL) * time.Minute,
		refreshTokenTTL: time.Duration(refreshTokenTTL) * time.Minute,
	}
}

func (s *JWTService) GenerateToken(userID, email string, tokenType TokenType) (string, error) {
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

func (s *JWTService) ValidateToken(tokenString string, expectedType TokenType) (*Claims, error) {
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

func (s *JWTService) GenerateTokenPair(userID, email string) (accessToken, refreshToken string, err error) {
	accessToken, err = s.GenerateToken(userID, email, AccessToken)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.GenerateToken(userID, email, RefreshToken)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
