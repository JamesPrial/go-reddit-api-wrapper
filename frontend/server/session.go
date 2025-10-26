package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

const (
	// tokenExpiryDuration is the duration for which JWT tokens are valid.
	tokenExpiryDuration = 24 * time.Hour

	// sessionCleanupInterval is how often to run session cleanup.
	sessionCleanupInterval = 1 * time.Hour

	// sessionMaxAge is the maximum age for a session before it's cleaned up.
	sessionMaxAge = 24 * time.Hour

	// jwtSecretMinLength is the minimum length for the JWT secret key.
	jwtSecretMinLength = 32
)

var (
	// jwtSecretKey holds the JWT secret loaded from environment.
	jwtSecretKey string
)

// initJWTSecret initializes the JWT secret from environment variable.
// It returns an error if the secret is not set or is too short.
func initJWTSecret() error {
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		return errors.New("JWT_SECRET_KEY environment variable is not set. " +
			"Generate one with: openssl rand -base64 32")
	}
	if len(secret) < jwtSecretMinLength {
		return fmt.Errorf("JWT_SECRET_KEY must be at least %d characters long, got %d",
			jwtSecretMinLength, len(secret))
	}
	jwtSecretKey = secret
	return nil
}

// Session represents a user session with a Reddit client.
type Session struct {
	SessionID      string
	RedditClient   *graw.Reddit
	Username       string
	CreatedAt      time.Time
	LastAccessedAt time.Time
}

// SessionManager manages user sessions with thread-safe operations.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	stopChan chan struct{}
}

// NewSessionManager creates a new SessionManager instance.
// It initializes the JWT secret from the environment and starts the cleanup goroutine.
func NewSessionManager() (*SessionManager, error) {
	// Initialize JWT secret from environment
	if err := initJWTSecret(); err != nil {
		return nil, err
	}

	sm := &SessionManager{
		sessions: make(map[string]*Session),
		stopChan: make(chan struct{}),
	}

	// Start background cleanup goroutine
	sm.startCleanup()

	return sm, nil
}

// CreateSession creates a new session for the given username and Reddit client.
// It returns the session ID and a JWT token for authentication.
func (sm *SessionManager) CreateSession(username string, client *graw.Reddit) (string, string, error) {
	if username == "" {
		return "", "", errors.New("username cannot be empty")
	}
	if client == nil {
		return "", "", errors.New("reddit client cannot be nil")
	}

	// Generate UUIDv4 session ID
	sessionID := uuid.New().String()

	// Create session with current timestamp for both creation and last access
	now := time.Now()
	session := &Session{
		SessionID:      sessionID,
		RedditClient:   client,
		Username:       username,
		CreatedAt:      now,
		LastAccessedAt: now,
	}

	// Store session
	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"session_id": sessionID,
		"username":   username,
		"exp":        now.Add(tokenExpiryDuration).Unix(),
		"iat":        now.Unix(),
	})

	tokenString, err := token.SignedString([]byte(jwtSecretKey))
	if err != nil {
		// Clean up session on token generation failure
		sm.mu.Lock()
		delete(sm.sessions, sessionID)
		sm.mu.Unlock()
		return "", "", fmt.Errorf("failed to sign JWT token: %w", err)
	}

	return sessionID, tokenString, nil
}

// GetSession retrieves a session by its ID and updates its last access time.
// Returns the session or an error if not found.
func (sm *SessionManager) GetSession(sessionID string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Update last accessed time on each access
	session.LastAccessedAt = time.Now()

	return session, nil
}

// DeleteSession removes a session by its ID.
func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, sessionID)
}

// ValidateJWT validates a JWT token and returns the session ID if valid.
// Returns an error if the token is invalid or expired.
func (sm *SessionManager) ValidateJWT(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecretKey), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse JWT token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token claims or token is not valid")
	}

	sessionID, ok := (*claims)["session_id"].(string)
	if !ok {
		return "", errors.New("session_id claim not found in token")
	}

	return sessionID, nil
}

// startCleanup starts a background goroutine that periodically cleans up expired sessions.
// Sessions are considered expired if they have not been accessed in the last sessionMaxAge duration.
func (sm *SessionManager) startCleanup() {
	go func() {
		ticker := time.NewTicker(sessionCleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				sm.cleanupExpiredSessions()
			case <-sm.stopChan:
				return
			}
		}
	}()
}

// cleanupExpiredSessions removes all sessions that have exceeded the maximum age.
// This method is called periodically to prevent unbounded memory growth.
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for sessionID, session := range sm.sessions {
		if now.Sub(session.LastAccessedAt) > sessionMaxAge {
			delete(sm.sessions, sessionID)
		}
	}
}

// Stop gracefully stops the session cleanup goroutine.
// Should be called during shutdown to ensure proper cleanup.
func (sm *SessionManager) Stop() {
	close(sm.stopChan)
}
