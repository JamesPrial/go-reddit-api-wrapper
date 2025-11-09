package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// TokenExpiredError is returned when a JWT token has expired.
type TokenExpiredError struct {
	Message string
}

// Error implements the error interface.
func (e *TokenExpiredError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "token has expired"
}

// TokenInvalidError is returned when a JWT token is invalid or cannot be parsed.
type TokenInvalidError struct {
	Message string
}

// Error implements the error interface.
func (e *TokenInvalidError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "invalid token"
}

// TokenMalformedError is returned when a JWT token is malformed.
type TokenMalformedError struct {
	Message string
}

// Error implements the error interface.
func (e *TokenMalformedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "malformed token"
}

// Claims represents the JWT claims for a user token.
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTService defines the interface for JWT token operations.
type JWTService interface {
	// GenerateToken creates a new JWT token for the user with the specified duration.
	// Returns the token string and the expiry time, or an error if generation fails.
	GenerateToken(user *User, duration time.Duration) (string, time.Time, error)

	// ValidateToken verifies and parses a JWT token string.
	// Returns the parsed claims if valid, or an error if invalid, expired, or malformed.
	ValidateToken(tokenString string) (*Claims, error)

	// RefreshToken validates an existing token and issues a new one with the specified duration.
	// Returns the new token string and expiry time, or an error if refresh fails.
	RefreshToken(tokenString string, duration time.Duration) (string, time.Time, error)
}

// jwtService implements JWTService using HS256 signing.
type jwtService struct {
	secretKey []byte
	issuer    string
}

// NewJWTService creates a new JWT service with the specified secret key and issuer.
// The secret key must be at least 32 bytes for HS256.
// Returns an error if the secret key is too short.
func NewJWTService(secretKey string, issuer string) (JWTService, error) {
	if secretKey == "" {
		return nil, errors.New("secret key cannot be empty")
	}

	if len([]byte(secretKey)) < 32 {
		return nil, fmt.Errorf("secret key must be at least 32 bytes (got %d)", len([]byte(secretKey)))
	}

	if issuer == "" {
		return nil, errors.New("issuer cannot be empty")
	}

	return &jwtService{
		secretKey: []byte(secretKey),
		issuer:    issuer,
	}, nil
}

// GenerateToken creates a new JWT token for the user.
// The token includes the username and role, along with standard JWT claims.
// The token expires after the specified duration.
func (s *jwtService) GenerateToken(user *User, duration time.Duration) (string, time.Time, error) {
	if user == nil {
		return "", time.Time{}, errors.New("user cannot be nil")
	}

	if duration <= 0 {
		return "", time.Time{}, errors.New("duration must be positive")
	}

	now := time.Now()
	expiresAt := now.Add(duration)

	claims := &Claims{
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   user.Username,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken verifies and parses a JWT token string.
// Returns the parsed claims if the token is valid, or an error otherwise.
// Validates the signature, expiration, issuer, and format.
func (s *jwtService) ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, &TokenInvalidError{Message: "token cannot be empty"}
	}

	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, &TokenInvalidError{
				Message: fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]),
			}
		}
		return s.secretKey, nil
	})

	if err != nil {
		// Check for specific JWT errors
		var ve *jwt.ValidationError
		if errors.As(err, &ve) {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return nil, &TokenMalformedError{Message: "token is malformed"}
			}
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, &TokenExpiredError{Message: "token has expired"}
			}
			if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				return nil, &TokenInvalidError{Message: "token is not valid yet"}
			}
		}
		return nil, &TokenInvalidError{Message: fmt.Sprintf("failed to parse token: %v", err)}
	}

	// Extract and validate claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, &TokenInvalidError{Message: "invalid token claims"}
	}

	// Validate issuer
	if claims.Issuer != s.issuer {
		return nil, &TokenInvalidError{
			Message: fmt.Sprintf("invalid issuer: expected %s, got %s", s.issuer, claims.Issuer),
		}
	}

	// Validate expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, &TokenExpiredError{Message: "token has expired"}
	}

	// Validate required fields
	if claims.Username == "" {
		return nil, &TokenInvalidError{Message: "token missing username claim"}
	}

	return claims, nil
}

// RefreshToken validates an existing token and issues a new one.
// The new token is issued with the specified duration and includes the same username and role.
// Returns the new token string and expiry time, or an error if refresh fails.
func (s *jwtService) RefreshToken(tokenString string, duration time.Duration) (string, time.Time, error) {
	// Validate the existing token
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", time.Time{}, err
	}

	// Create a temporary user from the claims for token generation
	tempUser := &User{
		Username: claims.Username,
		Role:     claims.Role,
	}

	// Generate a new token
	return s.GenerateToken(tempUser, duration)
}
