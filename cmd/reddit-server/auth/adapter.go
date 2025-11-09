package auth

import (
	"errors"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/handlers"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/middleware"
)

// HandlersUserStore adapts an InMemoryUserStore to implement the handlers.UserStore interface.
type HandlersUserStore struct {
	store *InMemoryUserStore
}

// NewHandlersUserStore creates a new adapter for the handlers package.
func NewHandlersUserStore(store *InMemoryUserStore) *HandlersUserStore {
	return &HandlersUserStore{store: store}
}

// ValidateCredentials validates user credentials and returns handlers.UserData.
// This method adapts the auth package's User type to the handlers package's UserData type.
func (s *HandlersUserStore) ValidateCredentials(username, password string) (*handlers.UserData, error) {
	user, err := s.store.ValidateCredentials(username, password)
	if err != nil {
		return nil, err
	}

	return &handlers.UserData{
		Username: user.Username,
		Role:     user.Role,
	}, nil
}

// HandlersJWTService adapts a jwtService to implement the handlers.JWTService interface.
type HandlersJWTService struct {
	service JWTService
}

// NewHandlersJWTService creates a new adapter for the handlers package.
func NewHandlersJWTService(service JWTService) *HandlersJWTService {
	return &HandlersJWTService{service: service}
}

// GenerateToken generates a JWT token for the given user data with the specified expiry time.
func (s *HandlersJWTService) GenerateToken(userData *handlers.UserData, expiresAt time.Time) (string, error) {
	if userData == nil {
		return "", errors.New("userData cannot be nil")
	}

	// Create a temporary user from the userData for token generation
	duration := time.Until(expiresAt)
	if duration <= 0 {
		return "", ErrExpiredToken
	}

	tempUser := &User{
		Username: userData.Username,
		Role:     userData.Role,
	}

	token, _, err := s.service.GenerateToken(tempUser, duration)
	return token, err
}

// ValidateToken validates a JWT token and returns the user data.
func (s *HandlersJWTService) ValidateToken(token string) (*handlers.UserData, error) {
	claims, err := s.service.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	return &handlers.UserData{
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}

// JWTValidatorAdapter adapts our JWTService to implement middleware.JWTAuthValidator interface.
type JWTValidatorAdapter struct {
	jwtService JWTService
}

// NewJWTValidatorAdapter creates a new adapter for the middleware package.
func NewJWTValidatorAdapter(service JWTService) *JWTValidatorAdapter {
	return &JWTValidatorAdapter{jwtService: service}
}

// ValidateToken validates a JWT token and returns middleware.UserClaims.
func (a *JWTValidatorAdapter) ValidateToken(tokenString string) (*middleware.UserClaims, error) {
	claims, err := a.jwtService.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return &middleware.UserClaims{
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}
