package auth

import (
	"testing"
	"time"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty string")
	}

	if hash == password {
		t.Error("HashPassword returned plaintext password")
	}

	// Verify hash starts with bcrypt prefix
	if len(hash) < 7 || (hash[:4] != "$2a$" && hash[:4] != "$2b$") {
		t.Errorf("Hash doesn't have bcrypt prefix: %s", hash[:7])
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testpassword123"
	hash, _ := HashPassword(password)

	// Test correct password
	if !VerifyPassword(password, hash) {
		t.Error("VerifyPassword failed for correct password")
	}

	// Test wrong password
	if VerifyPassword("wrongpassword", hash) {
		t.Error("VerifyPassword succeeded for wrong password")
	}

	// Test empty password
	if VerifyPassword("", hash) {
		t.Error("VerifyPassword succeeded for empty password")
	}
}

func TestNewUser(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		role        string
		expectError bool
	}{
		{"Valid admin", "admin", "password123", "admin", false},
		{"Valid viewer", "viewer", "password123", "viewer", false},
		{"Valid moderator", "moderator", "password123", "moderator", false},
		{"Invalid role", "user", "password123", "invalid", true},
		{"Empty username", "", "password123", "admin", true},
		{"Empty password", "admin", "", "admin", true},
		{"Empty role", "admin", "password123", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := NewUser(tt.username, tt.password, tt.role)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if user.Username != tt.username {
				t.Errorf("Username = %q, want %q", user.Username, tt.username)
			}

			if user.Role != tt.role {
				t.Errorf("Role = %q, want %q", user.Role, tt.role)
			}

			if !VerifyPassword(tt.password, user.PasswordHash) {
				t.Error("Password hash verification failed")
			}
		})
	}
}

func TestInMemoryUserStore_ValidateCredentials(t *testing.T) {
	user, _ := NewUser("testuser", "testpass", "admin")
	store := NewInMemoryUserStore([]*User{user})

	// Test valid credentials
	validatedUser, err := store.ValidateCredentials("testuser", "testpass")
	if err != nil {
		t.Fatalf("ValidateCredentials failed: %v", err)
	}
	if validatedUser.Username != "testuser" {
		t.Errorf("Username = %q, want %q", validatedUser.Username, "testuser")
	}

	// Test wrong password
	_, err = store.ValidateCredentials("testuser", "wrongpass")
	if err == nil {
		t.Error("Expected error for wrong password")
	}

	// Test non-existent user
	_, err = store.ValidateCredentials("nonexistent", "testpass")
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestInMemoryUserStore_AddUser(t *testing.T) {
	store := NewInMemoryUserStore(nil)

	user, _ := NewUser("newuser", "password", "viewer")

	// Test adding new user
	err := store.AddUser(user)
	if err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// Test adding duplicate user (should error now)
	err = store.AddUser(user)
	if err == nil {
		t.Error("Expected error when adding duplicate user")
	}

	// Test nil user
	err = store.AddUser(nil)
	if err == nil {
		t.Error("Expected error for nil user")
	}
}

func TestInMemoryUserStore_GetUser(t *testing.T) {
	user, _ := NewUser("testuser", "testpass", "admin")
	store := NewInMemoryUserStore([]*User{user})

	// Test getting existing user
	retrieved, err := store.GetUser("testuser")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.Username != "testuser" {
		t.Errorf("Username = %q, want %q", retrieved.Username, "testuser")
	}

	// Test getting non-existent user
	_, err = store.GetUser("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestInMemoryUserStore_ListUsers(t *testing.T) {
	user1, _ := NewUser("user1", "pass1", "admin")
	user2, _ := NewUser("user2", "pass2", "viewer")
	user3, _ := NewUser("user3", "pass3", "moderator")

	store := NewInMemoryUserStore([]*User{user1, user2, user3})

	users := store.ListUsers()
	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}

	// Verify all users are present (order may vary due to sync.Map)
	usernames := make(map[string]bool)
	for _, u := range users {
		usernames[u.Username] = true
	}

	expected := map[string]bool{"user1": true, "user2": true, "user3": true}
	for name := range expected {
		if !usernames[name] {
			t.Errorf("Expected user %q not found in list", name)
		}
	}
}

func TestNewUser_PasswordHashing(t *testing.T) {
	username := "testuser"
	password := "mypassword"
	role := "admin"

	user1, _ := NewUser(username, password, role)
	user2, _ := NewUser(username, password, role)

	// Same password should produce different hashes (due to salt)
	if user1.PasswordHash == user2.PasswordHash {
		t.Error("Identical passwords produced identical hashes (salt not working)")
	}

	// But both should verify correctly
	if !VerifyPassword(password, user1.PasswordHash) {
		t.Error("First user password hash doesn't verify")
	}
	if !VerifyPassword(password, user2.PasswordHash) {
		t.Error("Second user password hash doesn't verify")
	}
}

func TestNewUser_CreatedAt(t *testing.T) {
	before := time.Now()
	user, _ := NewUser("testuser", "password", "admin")
	after := time.Now()

	if user.CreatedAt.Before(before) || user.CreatedAt.After(after) {
		t.Errorf("CreatedAt timestamp is not within expected range: %v", user.CreatedAt)
	}
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	_, err := HashPassword("")
	if err == nil {
		t.Error("HashPassword should return error for empty password")
	}
}

func TestInMemoryUserStore_EmptyInitialization(t *testing.T) {
	store := NewInMemoryUserStore(nil)

	users := store.ListUsers()
	if len(users) != 0 {
		t.Errorf("Expected empty store, got %d users", len(users))
	}

	user, err := store.GetUser("nonexistent")
	if err == nil {
		t.Error("GetUser should return error for non-existent user")
	}
	if user != nil {
		t.Error("GetUser should return nil user for non-existent user")
	}
}

func TestInMemoryUserStore_NilUserHandling(t *testing.T) {
	// Verify that nil users in the init slice are skipped
	user1, _ := NewUser("user1", "pass1", "admin")
	var nilUser *User
	user2, _ := NewUser("user2", "pass2", "viewer")

	store := NewInMemoryUserStore([]*User{user1, nilUser, user2})

	users := store.ListUsers()
	if len(users) != 2 {
		t.Errorf("Expected 2 users (nil skipped), got %d", len(users))
	}
}
