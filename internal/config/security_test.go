package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tepzxl/contentflow/internal/config"
)

func TestLoadRejectsWeakJWTSecretOutsideDevelopment(t *testing.T) {
	path := writeTempConfig(t, `
app:
  env: prod
auth:
  jwt_secret: default secret
`)

	_, err := config.Load(path)

	if err == nil {
		t.Fatal("Load() expected weak JWT secret error, got nil")
	}
	if !strings.Contains(err.Error(), "weak jwt secret") {
		t.Fatalf("Load() error = %v, want weak jwt secret", err)
	}
}

func TestLoadAllowsDevelopmentJWTSecret(t *testing.T) {
	path := writeTempConfig(t, `
app:
  env: dev
auth:
  jwt_secret: default secret
`)

	if _, err := config.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadAppliesDatabaseDefaults(t *testing.T) {
	path := writeTempConfig(t, `
app:
  env: dev
auth:
  jwt_secret: default secret
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.Username != "contentflow" {
		t.Fatalf("Database.Username = %q, want contentflow", cfg.Database.Username)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Fatalf("Database.SSLMode = %q, want disable", cfg.Database.SSLMode)
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
