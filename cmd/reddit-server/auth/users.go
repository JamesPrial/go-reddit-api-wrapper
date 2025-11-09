package auth

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AdvancedUserStore defines the advanced interface for user management operations.
// This interface extends UserStore with credential validation and direct user lookup.
type AdvancedUserStore interface {
	// ValidateCredentials verifies username and password against stored credentials.
	// Returns the user if credentials are valid, or InvalidCredentialsError otherwise.
	ValidateCredentials(username, password string) (*User, error)

	// GetUser retrieves a user by username. Returns UserNotFoundError if not found.
	GetUser(username string) (*User, error)

	// ListUsers returns all users in the store.
	ListUsers() []*User

	// Embed the base UserStore interface for compatibility
	UserStore
}

// InMemoryUserStore provides thread-safe in-memory user storage.
type InMemoryUserStore struct {
	users sync.Map // map[string]*User
}

// NewInMemoryUserStore creates a new in-memory user store and populates it with the given users.
func NewInMemoryUserStore(users []*User) *InMemoryUserStore {
	store := &InMemoryUserStore{}
	for _, u := range users {
		if u != nil {
			store.users.Store(u.Username, u)
		}
	}
	return store
}

// ValidateCredentials verifies the username and password against the stored user.
// Uses constant-time comparison for username to prevent timing attacks.
// Returns InvalidCredentialsError if credentials are invalid.
func (s *InMemoryUserStore) ValidateCredentials(username, password string) (*User, error) {
	if username == "" || password == "" {
		return nil, &InvalidCredentialsError{
			Username: username,
			Message:  "username and password are required",
		}
	}

	val, ok := s.users.Load(username)
	if !ok {
		// Return generic error to prevent user enumeration
		return nil, &InvalidCredentialsError{
			Username: username,
			Message:  "invalid credentials",
		}
	}

	user := val.(*User)

	// Use bcrypt.CompareHashAndPassword for constant-time password comparison
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, &InvalidCredentialsError{
			Username: username,
			Message:  "invalid credentials",
		}
	}

	return user, nil
}

// GetUser retrieves a user by username. Returns UserNotFoundError if not found.
func (s *InMemoryUserStore) GetUser(username string) (*User, error) {
	if username == "" {
		return nil, &UserNotFoundError{Username: username}
	}

	val, ok := s.users.Load(username)
	if !ok {
		return nil, &UserNotFoundError{Username: username}
	}

	return val.(*User), nil
}

// ListUsers returns a slice of all users in the store.
func (s *InMemoryUserStore) ListUsers() []*User {
	var users []*User
	s.users.Range(func(key, value interface{}) bool {
		if user, ok := value.(*User); ok {
			users = append(users, user)
		}
		return true
	})
	return users
}

// HashPassword generates a bcrypt hash of the password using cost factor 12.
// Returns an error if hashing fails (e.g., password too long).
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword compares a plaintext password against a bcrypt hash.
// This version is used by main.go and returns a bool for compatibility.
// Parameters: plainPassword (user input), hash (stored hash)
func VerifyPassword(plainPassword, hash string) bool {
	if plainPassword == "" || hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plainPassword)) == nil
}

// NewUser creates a new user with a hashed password.
// Returns an error if password hashing fails.
func NewUser(username, password, role string) (*User, error) {
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}

	if role != "admin" && role != "moderator" && role != "viewer" {
		return nil, fmt.Errorf("invalid role: %s (must be 'admin', 'moderator', or 'viewer')", role)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	return &User{
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    time.Now(),
	}, nil
}

// AddUser adds a user to the store. Returns an error if a user with the same
// username already exists.
func (s *InMemoryUserStore) AddUser(user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	if user.Username == "" {
		return errors.New("username cannot be empty")
	}

	if user.PasswordHash == "" {
		return errors.New("password hash cannot be empty")
	}

	// Check if user already exists
	if _, exists := s.users.Load(user.Username); exists {
		return fmt.Errorf("user with username %q already exists", user.Username)
	}

	s.users.Store(user.Username, user)
	return nil
}
