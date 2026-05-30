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

func TestDockerConfigLoadsWithDevelopmentDefaults(t *testing.T) {
	t.Setenv("CONTENTFLOW_APP_ENV", "")
	t.Setenv("CONTENTFLOW_AUTH_JWT_SECRET", "")

	path := filepath.Join("..", "..", "configs", "config.docker.yaml")
	if _, err := config.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
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
