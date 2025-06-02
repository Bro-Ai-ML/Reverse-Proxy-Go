package auth

import "errors"

var (
	ErrInvalidToken          = errors.New("invalid or expired token")
	ErrInvalidSigningMethod  = errors.New("invalid token signing method")
	ErrMissingToken          = errors.New("missing authentication token")
	ErrInsufficientPrivilege = errors.New("insufficient privileges")
)
