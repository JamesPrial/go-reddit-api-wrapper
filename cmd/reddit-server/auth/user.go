// Package auth provides JWT authentication and user management for the Reddit server.
package auth

import (
	"fmt"
	"time"
)

// User represents an authenticated user in the system.
type User struct {
	Username     string    // Unique username
	PasswordHash string    // Bcrypt hash of password
	Role         string    // User role (e.g., "admin", "user")
	CreatedAt    time.Time // Account creation timestamp
}

// UserStore defines the interface for user storage operations.
// This interface is used by the userStoreAdapter in main.go.
type UserStore interface {
	// GetByUsername retrieves a user by username.
	// Returns ErrUserNotFound if the user does not exist.
	GetByUsername(username string) (*User, error)

	// ListAll returns all users in the store.
	ListAll() ([]*User, error)
}

// InvalidCredentialsError is returned when credentials cannot be validated.
type InvalidCredentialsError struct {
	Username string
	Message  string
}

// Error implements the error interface.
func (e *InvalidCredentialsError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "invalid credentials"
}

// UserNotFoundError is returned when a user cannot be found.
type UserNotFoundError struct {
	Username string
}

// Error implements the error interface.
func (e *UserNotFoundError) Error() string {
	return fmt.Sprintf("user not found: %s", e.Username)
}

// GetByUsername retrieves a user by username from the in-memory store.
// Returns ErrUserNotFound if the user does not exist.
func (s *InMemoryUserStore) GetByUsername(username string) (*User, error) {
	user, err := s.GetUser(username)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// ListAll returns all users in the store.
func (s *InMemoryUserStore) ListAll() ([]*User, error) {
	return s.ListUsers(), nil
}
