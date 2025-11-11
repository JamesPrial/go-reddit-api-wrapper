package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// UserConfig represents a user account configuration.
// It contains username, bcrypt password hash, and role assignment.
type UserConfig struct {
	Username     string `json:"username" yaml:"username"`           // Unique username for login
	PasswordHash string `json:"password_hash" yaml:"password_hash"` // Bcrypt password hash (starts with $2a$ or $2b$)
	Role         string `json:"role" yaml:"role"`                   // User role: "admin" or "viewer"
}

// AuthConfig contains authentication system configuration.
// It supports JWT-based session authentication with configurable token expiry
// and user credential management.
type AuthConfig struct {
	// Users is a list of user accounts
	Users []UserConfig `yaml:"users"`

	// JWTSecret is the secret key for signing JWT tokens (hex-encoded)
	// If empty, a cryptographically secure random secret is auto-generated
	JWTSecret string `yaml:"jwt_secret"`

	// TokenExpiry is the duration JWT tokens remain valid (default: 24h)
	TokenExpiry time.Duration `yaml:"token_expiry"`

	// Enabled indicates whether JWT auth is enabled
	Enabled bool `yaml:"enabled"`
}

// Constants for authentication configuration.
const (
	DefaultTokenExpiry = 24 * time.Hour
	JWTSecretLength    = 64 // bytes
	MinJWTSecretLength = 32 // bytes after hex decode
	MinTokenExpiry     = 1 * time.Hour
	MaxTokenExpiry     = 30 * 24 * time.Hour // 30 days
)

// Valid user roles.
var validRoles = map[string]bool{
	"admin":  true,
	"viewer": true,
}

// LoadAuthConfig loads authentication configuration from environment variables.
// It loads JWT_SECRET, TOKEN_EXPIRY, and USERS from the environment.
// If JWT_SECRET is empty, a cryptographically secure random secret is generated.
//
// Environment variables:
//   - AUTH_ENABLED: Enable JWT authentication (default: false)
//   - JWT_SECRET: Secret key for JWT signing (default: auto-generated if auth is enabled, hex-encoded)
//   - TOKEN_EXPIRY: JWT token expiry duration (default: 24h, accepts duration strings like "48h", "1h30m")
//   - USERS: JSON array of user objects [{"username":"user","password_hash":"$2a$...","role":"admin"}]
//
// Returns the auth config or an error if configuration is invalid.
// When auth is enabled, at least one user must be configured.
func LoadAuthConfig() (*AuthConfig, error) {
	cfg := &AuthConfig{
		Enabled:     false,
		TokenExpiry: DefaultTokenExpiry,
		Users:       []UserConfig{},
	}

	// Parse enabled flag
	if enabledStr := os.Getenv("AUTH_ENABLED"); enabledStr != "" {
		cfg.Enabled = strings.ToLower(enabledStr) == "true" || enabledStr == "1"
	}

	// Load JWT secret
	if secret := strings.TrimSpace(os.Getenv("JWT_SECRET")); secret != "" {
		cfg.JWTSecret = secret
	} else if cfg.Enabled {
		// Auto-generate secret if auth is enabled but secret not provided
		generated, err := generateJWTSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		cfg.JWTSecret = generated
	}

	// Parse token expiry
	if expiryStr := strings.TrimSpace(os.Getenv("TOKEN_EXPIRY")); expiryStr != "" {
		expiry, err := time.ParseDuration(expiryStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TOKEN_EXPIRY: %w", err)
		}
		cfg.TokenExpiry = expiry
	}

	// Parse users from USERS environment variable (JSON array)
	if usersStr := strings.TrimSpace(os.Getenv("USERS")); usersStr != "" {
		users, err := ParseUsers(usersStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse USERS: %w", err)
		}
		cfg.Users = users
	}

	return cfg, nil
}

// LoadAuthConfigFromFile loads authentication configuration from a file.
// It expects an authConfig YAML structure and returns a populated AuthConfig.
// This is typically called from LoadFromFile when processing YAML config.
// If JWT secret is empty and auth is enabled, one is auto-generated.
func LoadAuthConfigFromFile(fileAuth *fileAuthConfig) (*AuthConfig, error) {
	if fileAuth == nil {
		// Return config with defaults if no file auth config provided
		return &AuthConfig{
			Enabled:     false,
			TokenExpiry: DefaultTokenExpiry,
			Users:       []UserConfig{},
		}, nil
	}

	cfg := &AuthConfig{
		Enabled:     fileAuth.Enabled,
		TokenExpiry: DefaultTokenExpiry,
		Users:       fileAuth.Users,
	}

	// Use JWT secret from file, or generate if auth is enabled and secret is empty
	if fileAuth.JWTSecret != "" {
		cfg.JWTSecret = fileAuth.JWTSecret
	} else if cfg.Enabled {
		secret, err := generateJWTSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		cfg.JWTSecret = secret
	}

	// Parse token expiry if provided
	if fileAuth.TokenExpiry != "" {
		expiry, err := time.ParseDuration(fileAuth.TokenExpiry)
		if err != nil {
			return nil, fmt.Errorf("invalid token_expiry %q: %w", fileAuth.TokenExpiry, err)
		}
		cfg.TokenExpiry = expiry
	}

	return cfg, nil
}

// Validate checks that auth configuration is valid.
// If auth is disabled, validation always passes (auth is optional).
// If enabled, it validates:
//   - JWT secret is at least 32 bytes (after hex decode)
//   - Token expiry is between 1h and 30 days
//   - At least one user is configured
//   - Each user has non-empty username
//   - Each user has valid bcrypt password hash
//   - Each user has valid role ("admin" or "viewer")
//   - Usernames are unique
//
// Returns an error if any validation fails.
func (c *AuthConfig) Validate() error {
	if !c.Enabled {
		return nil // Auth is optional
	}

	var errs []error

	// Validate JWT secret if enabled
	if c.JWTSecret == "" {
		errs = append(errs, errors.New("JWT_SECRET is required when AUTH_ENABLED is true"))
	} else {
		// JWT secret should be hex-encoded
		secretBytes, err := hex.DecodeString(c.JWTSecret)
		if err != nil {
			errs = append(errs, fmt.Errorf("JWT secret is not valid hex encoding: %w", err))
		} else if len(secretBytes) < MinJWTSecretLength {
			errs = append(errs, fmt.Errorf("JWT secret must be at least %d bytes (got %d bytes)", MinJWTSecretLength, len(secretBytes)))
		}
	}

	// Validate token expiry
	if c.TokenExpiry <= 0 {
		errs = append(errs, fmt.Errorf("token expiry must be positive, got %v", c.TokenExpiry))
	}
	if c.TokenExpiry < MinTokenExpiry {
		errs = append(errs, fmt.Errorf("token expiry must be at least %v, got %v", MinTokenExpiry, c.TokenExpiry))
	}
	if c.TokenExpiry > MaxTokenExpiry {
		errs = append(errs, fmt.Errorf("token expiry must not exceed %v, got %v", MaxTokenExpiry, c.TokenExpiry))
	}

	// Validate users
	if len(c.Users) == 0 {
		errs = append(errs, errors.New("at least one user is required when AUTH_ENABLED is true"))
	}

	// Check for duplicate usernames and validate each user
	seen := make(map[string]bool)
	for i, user := range c.Users {
		if user.Username == "" {
			errs = append(errs, fmt.Errorf("user %d must have non-empty username", i+1))
		} else if seen[user.Username] {
			errs = append(errs, fmt.Errorf("user %d has duplicate username %q", i+1, user.Username))
		}
		seen[user.Username] = true

		// Validate password hash
		if err := ValidatePasswordHash(user.PasswordHash); err != nil {
			errs = append(errs, fmt.Errorf("user %d (%q): invalid password hash: %w", i+1, user.Username, err))
		}

		// Validate role
		if err := ValidateRole(user.Role); err != nil {
			errs = append(errs, fmt.Errorf("user %d (%q): %w", i+1, user.Username, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ParseUsers parses users from a JSON string (as provided in the USERS environment variable).
// The JSON string should be an array of user objects: [{"username":"u1","password_hash":"...","role":"admin"}]
// Returns error if JSON is invalid or malformed.
func ParseUsers(jsonStr string) ([]UserConfig, error) {
	var users []UserConfig
	if err := json.Unmarshal([]byte(jsonStr), &users); err != nil {
		return nil, fmt.Errorf("invalid JSON in USERS: %w", err)
	}
	return users, nil
}

// ValidatePasswordHash checks if a password hash is in valid bcrypt format.
// Valid bcrypt hashes start with $2a$ or $2b$ and are approximately 60 characters.
// Returns error if hash is invalid.
func ValidatePasswordHash(hash string) error {
	if hash == "" {
		return errors.New("password hash must not be empty")
	}
	if len(hash) < 60 {
		return fmt.Errorf("password hash is too short (expected ~60 chars, got %d)", len(hash))
	}
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		return errors.New("password hash must be in bcrypt format (start with $2a$ or $2b$)")
	}
	return nil
}

// ValidateRole checks if a role is valid.
// Valid roles are "admin" and "viewer".
// Returns error if role is invalid.
func ValidateRole(role string) error {
	if role == "" {
		return errors.New("role must not be empty")
	}
	if !validRoles[role] {
		return fmt.Errorf("invalid role %q (must be one of: admin, viewer)", role)
	}
	return nil
}

// generateJWTSecret generates a cryptographically secure random JWT secret.
// It generates 64 random bytes and encodes them as a hex string.
// Returns error if random generation fails.
func generateJWTSecret() (string, error) {
	buf := make([]byte, JWTSecretLength)
	n, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	if n != len(buf) {
		return "", fmt.Errorf("insufficient random bytes: got %d, want %d", n, len(buf))
	}
	return hex.EncodeToString(buf), nil
}

// String returns a string representation of the auth configuration for logging.
// It redacts sensitive credentials (JWT secret and password hashes).
func (c *AuthConfig) String() string {
	userStrs := make([]string, len(c.Users))
	for i, u := range c.Users {
		userStrs[i] = fmt.Sprintf("{username:%q, role:%q}", u.Username, u.Role)
	}
	return fmt.Sprintf(
		"AuthConfig{Enabled: %v, JWTSecret: %s, TokenExpiry: %v, Users: [%s]}",
		c.Enabled,
		redact(c.JWTSecret),
		c.TokenExpiry,
		strings.Join(userStrs, ", "),
	)
}

// fileAuthConfig represents the authentication section of the YAML configuration file.
type fileAuthConfig struct {
	Enabled     bool         `yaml:"enabled"`
	JWTSecret   string       `yaml:"jwt_secret"`
	TokenExpiry string       `yaml:"token_expiry"`
	Users       []UserConfig `yaml:"users"`
}
