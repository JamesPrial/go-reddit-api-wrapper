package config

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// TestLoadAuthConfig_Disabled tests loading auth config with auth disabled.
func TestLoadAuthConfig_Disabled(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("TOKEN_EXPIRY", "")
	t.Setenv("USERS", "")

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("LoadAuthConfig() failed: %v", err)
	}

	if cfg.Enabled {
		t.Error("Auth should be disabled")
	}

	// JWT secret should not be auto-generated if auth is disabled
	if cfg.JWTSecret != "" {
		t.Error("JWT secret should not be generated when auth is disabled")
	}

	if cfg.TokenExpiry != DefaultTokenExpiry {
		t.Errorf("TokenExpiry = %v, want %v", cfg.TokenExpiry, DefaultTokenExpiry)
	}

	if len(cfg.Users) != 0 {
		t.Errorf("Users should be empty, got %d", len(cfg.Users))
	}
}

// TestLoadAuthConfig_EnabledNoSecret tests loading with auth enabled but no secret.
func TestLoadAuthConfig_EnabledNoSecret(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("TOKEN_EXPIRY", "")
	t.Setenv("USERS", "")

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("LoadAuthConfig() failed: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Auth should be enabled")
	}

	// JWT secret should be auto-generated
	if cfg.JWTSecret == "" {
		t.Error("JWT secret should be auto-generated")
	}

	// Should be valid hex
	_, err = hex.DecodeString(cfg.JWTSecret)
	if err != nil {
		t.Errorf("JWT secret is not valid hex: %v", err)
	}
}

// TestLoadAuthConfig_WithJWTSecret tests loading with JWT secret from environment.
func TestLoadAuthConfig_WithJWTSecret(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("JWT_SECRET", secret)
	t.Setenv("TOKEN_EXPIRY", "")
	t.Setenv("USERS", "")

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("LoadAuthConfig() failed: %v", err)
	}

	if cfg.JWTSecret != secret {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, secret)
	}
}

// TestLoadAuthConfig_WithTokenExpiry tests loading with custom token expiry.
func TestLoadAuthConfig_WithTokenExpiry(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("TOKEN_EXPIRY", "48h")
	t.Setenv("USERS", "")

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("LoadAuthConfig() failed: %v", err)
	}

	expectedExpiry := 48 * time.Hour
	if cfg.TokenExpiry != expectedExpiry {
		t.Errorf("TokenExpiry = %v, want %v", cfg.TokenExpiry, expectedExpiry)
	}
}

// TestLoadAuthConfig_WithInvalidTokenExpiry tests loading with invalid token expiry format.
func TestLoadAuthConfig_WithInvalidTokenExpiry(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("TOKEN_EXPIRY", "invalid")
	t.Setenv("USERS", "")

	cfg, err := LoadAuthConfig()
	if err == nil {
		t.Fatalf("LoadAuthConfig() should fail with invalid TOKEN_EXPIRY")
	}
	if cfg != nil {
		t.Errorf("cfg should be nil on error, got %v", cfg)
	}
	if !strings.Contains(err.Error(), "invalid TOKEN_EXPIRY") {
		t.Errorf("error should mention TOKEN_EXPIRY, got: %v", err)
	}
}

// TestLoadAuthConfig_WithUsers tests loading with users from environment.
func TestLoadAuthConfig_WithUsers(t *testing.T) {
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"
	usersJSON := `[{"username":"admin","password_hash":"` + validHash + `","role":"admin"}]`

	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("TOKEN_EXPIRY", "")
	t.Setenv("USERS", usersJSON)

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("LoadAuthConfig() failed: %v", err)
	}

	if len(cfg.Users) != 1 {
		t.Fatalf("Users should have 1 entry, got %d", len(cfg.Users))
	}

	if cfg.Users[0].Username != "admin" {
		t.Errorf("Username = %q, want %q", cfg.Users[0].Username, "admin")
	}
	if cfg.Users[0].Role != "admin" {
		t.Errorf("Role = %q, want %q", cfg.Users[0].Role, "admin")
	}
}

// TestLoadAuthConfig_WithInvalidUsersJSON tests loading with invalid users JSON.
func TestLoadAuthConfig_WithInvalidUsersJSON(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("TOKEN_EXPIRY", "")
	t.Setenv("USERS", "invalid json")

	cfg, err := LoadAuthConfig()
	if err == nil {
		t.Fatalf("LoadAuthConfig() should fail with invalid USERS JSON")
	}
	if cfg != nil {
		t.Errorf("cfg should be nil on error, got %v", cfg)
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("error should mention JSON, got: %v", err)
	}
}

// TestAuthConfig_Validate_DisabledAlwaysPasses tests that disabled auth always validates.
func TestAuthConfig_Validate_DisabledAlwaysPasses(t *testing.T) {
	cfg := &AuthConfig{
		Enabled:     false,
		JWTSecret:   "",
		TokenExpiry: 0,
		Users:       []UserConfig{},
	}

	// Should always pass even with invalid values
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() should pass when auth is disabled, got: %v", err)
	}
}

// TestAuthConfig_Validate_Success tests validation of a valid auth config.
func TestAuthConfig_Validate_Success(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"

	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: 24 * time.Hour,
		Users: []UserConfig{
			{
				Username:     "admin",
				PasswordHash: validHash,
				Role:         "admin",
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
}

// TestAuthConfig_Validate_InvalidJWTSecret tests validation with invalid JWT secret.
func TestAuthConfig_Validate_InvalidJWTSecret(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		secret  string
	}{
		{
			name:    "empty when enabled",
			enabled: true,
			secret:  "",
		},
		{
			name:    "not hex",
			enabled: true,
			secret:  "not-hex-string-this-is-clearly-invalid",
		},
		{
			name:    "too short",
			enabled: true,
			secret:  hex.EncodeToString([]byte("short")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"
			cfg := &AuthConfig{
				Enabled:     tt.enabled,
				JWTSecret:   tt.secret,
				TokenExpiry: 24 * time.Hour,
				Users: []UserConfig{
					{
						Username:     "admin",
						PasswordHash: validHash,
						Role:         "admin",
					},
				},
			}

			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() should fail with invalid JWT secret")
			}
		})
	}
}

// TestAuthConfig_Validate_InvalidTokenExpiry tests validation with invalid token expiry.
func TestAuthConfig_Validate_InvalidTokenExpiry(t *testing.T) {
	tests := []struct {
		name   string
		expiry time.Duration
	}{
		{
			name:   "too short",
			expiry: 30 * time.Minute,
		},
		{
			name:   "too long",
			expiry: 31 * 24 * time.Hour,
		},
		{
			name:   "zero",
			expiry: 0,
		},
		{
			name:   "negative",
			expiry: -1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
			validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"

			cfg := &AuthConfig{
				Enabled:     true,
				JWTSecret:   secret,
				TokenExpiry: tt.expiry,
				Users: []UserConfig{
					{
						Username:     "admin",
						PasswordHash: validHash,
						Role:         "admin",
					},
				},
			}

			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() should fail with invalid token expiry")
			}
		})
	}
}

// TestAuthConfig_Validate_NoUsers tests validation with no users configured.
func TestAuthConfig_Validate_NoUsers(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: 24 * time.Hour,
		Users:       []UserConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("Validate() should fail with no users")
	}
	if !strings.Contains(err.Error(), "at least one user") {
		t.Errorf("error should mention users, got: %v", err)
	}
}

// TestAuthConfig_Validate_DuplicateUsernames tests validation with duplicate usernames.
func TestAuthConfig_Validate_DuplicateUsernames(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"

	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: 24 * time.Hour,
		Users: []UserConfig{
			{Username: "admin", PasswordHash: validHash, Role: "admin"},
			{Username: "admin", PasswordHash: validHash, Role: "viewer"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("Validate() should fail with duplicate usernames")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error should mention duplicate, got: %v", err)
	}
}

// TestParseUsers tests parsing users from JSON.
func TestParseUsers(t *testing.T) {
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"
	json := `[
		{"username":"admin","password_hash":"` + validHash + `","role":"admin"},
		{"username":"viewer","password_hash":"` + validHash + `","role":"viewer"}
	]`

	users, err := ParseUsers(json)
	if err != nil {
		t.Fatalf("ParseUsers() failed: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(users))
	}

	if users[0].Username != "admin" {
		t.Errorf("users[0].Username = %q, want admin", users[0].Username)
	}
	if users[1].Username != "viewer" {
		t.Errorf("users[1].Username = %q, want viewer", users[1].Username)
	}
}

// TestParseUsers_Invalid tests parsing invalid JSON.
func TestParseUsers_Invalid(t *testing.T) {
	_, err := ParseUsers("invalid json")
	if err == nil {
		t.Fatalf("ParseUsers() should fail with invalid JSON")
	}
}

// TestParseUsers_Empty tests parsing empty array.
func TestParseUsers_Empty(t *testing.T) {
	users, err := ParseUsers("[]")
	if err != nil {
		t.Fatalf("ParseUsers() failed: %v", err)
	}

	if len(users) != 0 {
		t.Errorf("Expected 0 users, got %d", len(users))
	}
}

// TestValidatePasswordHash tests password hash validation.
func TestValidatePasswordHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{
			name:    "valid 2a",
			hash:    "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG",
			wantErr: false,
		},
		{
			name:    "valid 2b",
			hash:    "$2b$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG",
			wantErr: false,
		},
		{
			name:    "empty",
			hash:    "",
			wantErr: true,
		},
		{
			name:    "too short",
			hash:    "$2a$12$abc",
			wantErr: true,
		},
		{
			name:    "invalid prefix 2c",
			hash:    "$2c$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG",
			wantErr: true,
		},
		{
			name:    "invalid prefix 2x",
			hash:    "$2x$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG",
			wantErr: true,
		},
		{
			name:    "no prefix",
			hash:    "R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordHash(tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePasswordHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateRole tests role validation.
func TestValidateRole(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		wantErr bool
	}{
		{name: "admin", role: "admin", wantErr: false},
		{name: "viewer", role: "viewer", wantErr: false},
		{name: "empty", role: "", wantErr: true},
		{name: "invalid", role: "superuser", wantErr: true},
		{name: "case sensitive", role: "Admin", wantErr: true},
		{name: "whitespace", role: " admin ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRole(tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGenerateJWTSecret tests JWT secret generation.
func TestGenerateJWTSecret(t *testing.T) {
	secret, err := generateJWTSecret()
	if err != nil {
		t.Fatalf("generateJWTSecret() failed: %v", err)
	}

	// Should be non-empty
	if secret == "" {
		t.Error("JWT secret should not be empty")
	}

	// Should be valid hex
	bytes, err := hex.DecodeString(secret)
	if err != nil {
		t.Errorf("JWT secret is not valid hex: %v", err)
	}

	// Should be 64 bytes
	if len(bytes) != JWTSecretLength {
		t.Errorf("JWT secret length = %d, want %d", len(bytes), JWTSecretLength)
	}
}

// TestGenerateJWTSecret_Uniqueness tests that multiple calls generate different secrets.
func TestGenerateJWTSecret_Uniqueness(t *testing.T) {
	secret1, err := generateJWTSecret()
	if err != nil {
		t.Fatalf("generateJWTSecret() failed: %v", err)
	}

	secret2, err := generateJWTSecret()
	if err != nil {
		t.Fatalf("generateJWTSecret() failed: %v", err)
	}

	if secret1 == secret2 {
		t.Errorf("generateJWTSecret() should generate unique secrets")
	}
}

// TestAuthConfig_String tests the string representation (should redact secrets).
func TestAuthConfig_String(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"

	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: 24 * time.Hour,
		Users: []UserConfig{
			{
				Username:     "admin",
				PasswordHash: validHash,
				Role:         "admin",
			},
		},
	}

	str := cfg.String()

	// JWT secret should be redacted
	if !strings.Contains(str, "<redacted>") {
		t.Errorf("String() should redact JWT secret, got: %s", str)
	}

	// Secret value should not appear in plain text
	if strings.Contains(str, secret) {
		t.Errorf("String() should not contain plain JWT secret")
	}

	// Password hash should not appear in plain text
	if strings.Contains(str, validHash) {
		t.Errorf("String() should not contain plain password hash")
	}

	// Usernames should be visible
	if !strings.Contains(str, "admin") {
		t.Errorf("String() should contain username, got: %s", str)
	}
}

// TestLoadAuthConfigFromFile tests loading from file configuration.
func TestLoadAuthConfigFromFile_WithValidConfig(t *testing.T) {
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))

	fileAuth := &fileAuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: "48h",
		Users: []UserConfig{
			{
				Username:     "admin",
				PasswordHash: validHash,
				Role:         "admin",
			},
		},
	}

	cfg, err := LoadAuthConfigFromFile(fileAuth)
	if err != nil {
		t.Fatalf("LoadAuthConfigFromFile() failed: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Auth should be enabled")
	}

	if cfg.JWTSecret != secret {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, secret)
	}

	if cfg.TokenExpiry != 48*time.Hour {
		t.Errorf("TokenExpiry = %v, want %v", cfg.TokenExpiry, 48*time.Hour)
	}

	if len(cfg.Users) != 1 || cfg.Users[0].Username != "admin" {
		t.Errorf("Users not loaded correctly")
	}
}

// TestLoadAuthConfigFromFile_Nil tests loading with nil file config.
func TestLoadAuthConfigFromFile_Nil(t *testing.T) {
	cfg, err := LoadAuthConfigFromFile(nil)
	if err != nil {
		t.Fatalf("LoadAuthConfigFromFile(nil) failed: %v", err)
	}

	if cfg.Enabled {
		t.Error("Auth should be disabled by default")
	}

	// Should have default expiry
	if cfg.TokenExpiry != DefaultTokenExpiry {
		t.Errorf("TokenExpiry = %v, want %v", cfg.TokenExpiry, DefaultTokenExpiry)
	}
}

// TestLoadAuthConfigFromFile_GeneratesSecretWhenEnabled tests auto-generation in file loading.
func TestLoadAuthConfigFromFile_GeneratesSecretWhenEnabled(t *testing.T) {
	fileAuth := &fileAuthConfig{
		Enabled:     true,
		JWTSecret:   "",
		TokenExpiry: "24h",
		Users: []UserConfig{
			{Username: "admin", PasswordHash: "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG", Role: "admin"},
		},
	}

	cfg, err := LoadAuthConfigFromFile(fileAuth)
	if err != nil {
		t.Fatalf("LoadAuthConfigFromFile() failed: %v", err)
	}

	// Should have auto-generated secret
	if cfg.JWTSecret == "" {
		t.Error("JWTSecret should be auto-generated")
	}

	// Should be valid hex
	_, err = hex.DecodeString(cfg.JWTSecret)
	if err != nil {
		t.Errorf("JWT secret is not valid hex: %v", err)
	}
}

// TestLoadAuthConfigFromFile_InvalidTokenExpiry tests with invalid duration in file.
func TestLoadAuthConfigFromFile_InvalidTokenExpiry(t *testing.T) {
	fileAuth := &fileAuthConfig{
		Enabled:     true,
		JWTSecret:   "",
		TokenExpiry: "invalid",
		Users:       []UserConfig{},
	}

	cfg, err := LoadAuthConfigFromFile(fileAuth)
	if err == nil {
		t.Fatalf("LoadAuthConfigFromFile() should fail with invalid token_expiry")
	}
	if cfg != nil {
		t.Errorf("cfg should be nil on error")
	}
	if !strings.Contains(err.Error(), "invalid token_expiry") {
		t.Errorf("error should mention token_expiry, got: %v", err)
	}
}

// TestAuthConfig_Validate_AllErrors tests that validate returns all errors at once.
func TestAuthConfig_Validate_AllErrors(t *testing.T) {
	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   "invalid",
		TokenExpiry: 0,
		Users:       []UserConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("Validate() should fail with multiple errors")
	}

	errStr := err.Error()
	// Should contain multiple error messages
	if !strings.Contains(errStr, "JWT secret") {
		t.Errorf("error should mention JWT secret")
	}
	if !strings.Contains(errStr, "token expiry") {
		t.Errorf("error should mention token expiry")
	}
	if !strings.Contains(errStr, "at least one user") {
		t.Errorf("error should mention users")
	}
}

// TestAuthConfig_Validate_EmptyUsername tests validation with empty username.
func TestAuthConfig_Validate_EmptyUsername(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"

	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: 24 * time.Hour,
		Users: []UserConfig{
			{Username: "", PasswordHash: validHash, Role: "admin"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() should fail with empty username")
	}
}

// TestAuthConfig_Validate_InvalidPasswordHash tests validation with invalid password hash.
func TestAuthConfig_Validate_InvalidPasswordHash(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))

	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: 24 * time.Hour,
		Users: []UserConfig{
			{Username: "admin", PasswordHash: "invalid", Role: "admin"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() should fail with invalid password hash")
	}
}

// TestAuthConfig_Validate_InvalidRole tests validation with invalid role.
func TestAuthConfig_Validate_InvalidRole(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"

	cfg := &AuthConfig{
		Enabled:     true,
		JWTSecret:   secret,
		TokenExpiry: 24 * time.Hour,
		Users: []UserConfig{
			{Username: "admin", PasswordHash: validHash, Role: "superuser"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() should fail with invalid role")
	}
}

// TestAuthConfig_TokenExpiry_Boundaries tests token expiry at boundary values.
func TestAuthConfig_TokenExpiry_Boundaries(t *testing.T) {
	secret := hex.EncodeToString([]byte("this-is-a-64-character-secret-that-is-long-enough-for-testing-"))
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"

	tests := []struct {
		name    string
		expiry  time.Duration
		wantErr bool
	}{
		{name: "at minimum", expiry: MinTokenExpiry, wantErr: false},
		{name: "just below minimum", expiry: MinTokenExpiry - 1*time.Minute, wantErr: true},
		{name: "at maximum", expiry: MaxTokenExpiry, wantErr: false},
		{name: "just above maximum", expiry: MaxTokenExpiry + 1*time.Hour, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AuthConfig{
				Enabled:     true,
				JWTSecret:   secret,
				TokenExpiry: tt.expiry,
				Users: []UserConfig{
					{Username: "admin", PasswordHash: validHash, Role: "admin"},
				},
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkGenerateJWTSecret benchmarks JWT secret generation.
func BenchmarkGenerateJWTSecret(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = generateJWTSecret()
	}
}

// BenchmarkValidatePasswordHash benchmarks password hash validation.
func BenchmarkValidatePasswordHash(b *testing.B) {
	hash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidatePasswordHash(hash)
	}
}

// BenchmarkParseUsers benchmarks user parsing.
func BenchmarkParseUsers(b *testing.B) {
	validHash := "$2a$12$R9h/cIPz0gi.URNNGHQ1Ke3T0H3w+WbSz/gXIl3s6i4j3eFXfWEBG"
	usersJSON := `[{"username":"admin","password_hash":"` + validHash + `","role":"admin"},{"username":"viewer","password_hash":"` + validHash + `","role":"viewer"}]`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseUsers(usersJSON)
	}
}
