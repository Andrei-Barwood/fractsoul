package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvPrefersDirectValueOverFile(t *testing.T) {
	tmpDir := t.TempDir()
	secretPath := filepath.Join(tmpDir, "database_url.txt")
	if err := os.WriteFile(secretPath, []byte("postgres://from-file"), 0o600); err != nil {
		t.Fatalf("write temp secret file: %v", err)
	}

	t.Setenv("DATABASE_URL", "postgres://from-env")
	t.Setenv("DATABASE_URL_FILE", secretPath)

	got := getEnv("DATABASE_URL", "")
	if got != "postgres://from-env" {
		t.Fatalf("expected env value to win, got %q", got)
	}
}

func TestGetEnvLoadsFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	secretPath := filepath.Join(tmpDir, "database_url.txt")
	if err := os.WriteFile(secretPath, []byte("postgres://from-file\n"), 0o600); err != nil {
		t.Fatalf("write temp secret file: %v", err)
	}

	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_URL_FILE", secretPath)

	got := getEnv("DATABASE_URL", "")
	if got != "postgres://from-file" {
		t.Fatalf("expected file value, got %q", got)
	}
}

func TestGetEnvAsKeyRoleMap(t *testing.T) {
	t.Setenv("API_KEY_ROLES", "viewer-key:viewer,operator-key:operator,admin-key:admin,broken:owner")

	got := getEnvAsKeyRoleMap("API_KEY_ROLES")
	if len(got) != 3 {
		t.Fatalf("expected 3 valid role mappings, got %d (%v)", len(got), got)
	}
	if got["viewer-key"] != "viewer" {
		t.Fatalf("unexpected viewer role mapping: %v", got["viewer-key"])
	}
	if got["operator-key"] != "operator" {
		t.Fatalf("unexpected operator role mapping: %v", got["operator-key"])
	}
	if got["admin-key"] != "admin" {
		t.Fatalf("unexpected admin role mapping: %v", got["admin-key"])
	}
}

func TestDefaultAPIRoleFallback(t *testing.T) {
	if role := defaultAPIRole("owner"); role != "admin" {
		t.Fatalf("expected fallback admin role, got %q", role)
	}
}
