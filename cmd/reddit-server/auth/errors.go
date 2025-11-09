package auth

import "errors"

var (
	// ErrUserNotFound is returned when a user cannot be found in the store.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidCredentials is returned when credentials do not match.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrInvalidToken is returned when a token is malformed or invalid.
	ErrInvalidToken = errors.New("invalid token")

	// ErrExpiredToken is returned when a token has expired.
	ErrExpiredToken = errors.New("token expired")

	// ErrInvalidSecret is returned when JWT secret is invalid.
	ErrInvalidSecret = errors.New("invalid JWT secret")

	// ErrTokenGeneration is returned when token generation fails.
	ErrTokenGeneration = errors.New("token generation failed")

	// ErrInvalidDuration is returned when token duration is invalid.
	ErrInvalidDuration = errors.New("invalid token duration")
)
