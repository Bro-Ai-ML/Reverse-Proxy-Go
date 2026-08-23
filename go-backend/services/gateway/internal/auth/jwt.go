package auth

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	signingMethod jwt.SigningMethod
	issuer        string
	audience      []string
}

type Claims struct {
	UserID string   `json:"uid"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

func NewJWTManager(privateKey []byte, publicKey []byte, issuer string, audience []string) (*JWTManager, error) {
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		return nil, err
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(publicKey)
	if err != nil {
		return nil, err
	}
	return &JWTManager{
		privateKey:    privKey,
		publicKey:     pubKey,
		signingMethod: jwt.SigningMethodRS256,
		issuer:        issuer,
		audience:      audience,
	}, nil
}

func (j *JWTManager) GenerateToken(userID string, roles []string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userID,
			Audience:  j.audience,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(j.signingMethod, claims)
	return token.SignedString(j.privateKey)
}

func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{j.signingMethod.Alg()})}
	if j.issuer != "" {
		opts = append(opts, jwt.WithIssuer(j.issuer))
	}
	if len(j.audience) > 0 {
		// Enforce the audience: tokens minted for other services must be
		// rejected (previously the audience was written but never checked).
		opts = append(opts, jwt.WithAudience(j.audience[0]))
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != j.signingMethod {
			return nil, errors.New("invalid signing method")
		}
		return j.publicKey, nil
	}, opts...)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}
