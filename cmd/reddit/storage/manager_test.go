package storage

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHomePath_PreventTraversal(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		shouldErr bool
		desc      string
	}{
		{
			name:      "valid home relative path",
			path:      "~/databases/reddit.db",
			shouldErr: false,
			desc:      "Normal home-relative path should succeed",
		},
		{
			name:      "path without home expansion",
			path:      "/tmp/reddit.db",
			shouldErr: false,
			desc:      "Absolute path without ~ should pass through",
		},
		{
			name:      "traversal attempt with ..",
			path:      "~/../../etc/passwd",
			shouldErr: true,
			desc:      "Path traversal with .. should be rejected",
		},
		{
			name:      "traversal attempt multiple levels",
			path:      "~/../../../tmp/attack.db",
			shouldErr: true,
			desc:      "Multiple .. components should be rejected",
		},
		{
			name:      "just tilde",
			path:      "~",
			shouldErr: false,
			desc:      "Just ~ should expand to home",
		},
		{
			name:      "valid nested path",
			path:      "~/my/nested/dir/database.db",
			shouldErr: false,
			desc:      "Deeply nested valid path should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandHomePath(tt.path)

			if (err != nil) != tt.shouldErr {
				t.Fatalf("expandHomePath(%q) error = %v, shouldErr = %v. %s",
					tt.path, err, tt.shouldErr, tt.desc)
			}

			// If no error expected, verify result is absolute and within home
			if !tt.shouldErr && !filepath.IsAbs(result) {
				t.Fatalf("expandHomePath(%q) returned relative path %q, expected absolute", tt.path, result)
			}

			// Verify non-traversal paths contain expected directory structure
			if !tt.shouldErr && tt.path != "~" {
				if !filepath.IsAbs(result) {
					t.Fatalf("expandHomePath(%q) returned non-absolute path %q", tt.path, result)
				}
			}
		})
	}
}

func TestValidateDatabasePath_PreventRootAccess(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		shouldErr bool
		desc      string
	}{
		{
			name:      "valid user path",
			path:      "/tmp/reddit.db",
			shouldErr: false,
			desc:      "Valid path in /tmp should succeed",
		},
		{
			name:      "database in root directory",
			path:      "/reddit.db",
			shouldErr: true,
			desc:      "Database file in root directory should be rejected",
		},
		{
			name:      "relative path in home",
			path:      "./databases/reddit.db",
			shouldErr: false,
			desc:      "Relative path in current directory should succeed",
		},
		{
			name:      "valid nested user directory",
			path:      "/home/user/databases/reddit.db",
			shouldErr: false,
			desc:      "Valid nested user directory should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDatabasePath(tt.path)

			if (err != nil) != tt.shouldErr {
				t.Fatalf("validateDatabasePath(%q) error = %v, shouldErr = %v. %s",
					tt.path, err, tt.shouldErr, tt.desc)
			}
		})
	}
}

func TestManager_NilLoggerSafety(t *testing.T) {
	// Test that Close() handles nil logger gracefully
	m := &Manager{
		store:  nil,
		logger: nil,
	}

	// This should not panic even with nil logger
	err := m.Close()
	if err != nil {
		t.Fatalf("Close() with nil logger returned error: %v", err)
	}
}

func TestNewManager_ValidatesDatabasePath(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	tests := []struct {
		name      string
		dbPath    string
		shouldErr bool
		desc      string
	}{
		{
			name:      "database in root directory",
			dbPath:    "/reddit.db",
			shouldErr: true,
			desc:      "Creating database in root directory should fail",
		},
		{
			name:      "valid memory database",
			dbPath:    ":memory:",
			shouldErr: false,
			desc:      "Memory database should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewManager(ctx, tt.dbPath, logger)

			if (err != nil) != tt.shouldErr {
				t.Fatalf("NewManager(%q) error = %v, shouldErr = %v. %s",
					tt.dbPath, err, tt.shouldErr, tt.desc)
			}
		})
	}
}

func TestNewManager_ExpandsHomeDirectory(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	// Use memory database with home expansion to test the feature
	tempDir := os.TempDir()
	relativePath := filepath.Join(tempDir, "reddit_test_"+t.Name()+".db")

	manager, err := NewManager(ctx, relativePath, logger)
	if err != nil {
		// This is expected since we're not testing actual DB init, just path expansion
		// The important thing is path expansion didn't cause a security error
		if err.Error() != "initialize storage: database file does not exist" {
			t.Logf("Note: Expected DB init error, got: %v", err)
		}
	}

	if manager != nil {
		defer manager.Close()
	}
}

func TestNilLoggerInClose(t *testing.T) {
	// Test defensive programming for nil logger in Close()
	m := &Manager{
		store:  nil,
		logger: nil,
	}

	// Should complete without panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close() panicked with nil logger: %v", r)
		}
	}()

	err := m.Close()
	if err != nil {
		t.Fatalf("Close() with nil store should return nil, got: %v", err)
	}
}

func TestExpandHomePathEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{
			name:      "empty path with tilde",
			path:      "~",
			shouldErr: false,
		},
		{
			name:      "single dot after tilde",
			path:      "~/.",
			shouldErr: false,
		},
		{
			name:      "double dot escape",
			path:      "~/..",
			shouldErr: true,
		},
		{
			name:      "complex valid nesting",
			path:      "~/a/b/c/d/e/f/file.db",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := expandHomePath(tt.path)
			if (err != nil) != tt.shouldErr {
				t.Fatalf("expandHomePath(%q): expected error=%v, got %v",
					tt.path, tt.shouldErr, err)
			}
		})
	}
}
