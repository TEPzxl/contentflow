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

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("CONTENTFLOW_APP_MODE", "worker")
	t.Setenv("CONTENTFLOW_DATABASE_PASSWORD", "from-env")
	t.Setenv("CONTENTFLOW_KAFKA_BROKERS", "kafka:9092,localhost:9092")
	t.Setenv("CONTENTFLOW_AUTH_JWT_SECRET", "env-secret")

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

	if cfg.App.Mode != "worker" {
		t.Fatalf("App.Mode = %q, want worker", cfg.App.Mode)
	}
	if cfg.Database.Password != "from-env" {
		t.Fatalf("Database.Password = %q, want from-env", cfg.Database.Password)
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "kafka:9092" || cfg.Kafka.Brokers[1] != "localhost:9092" {
		t.Fatalf("Kafka.Brokers = %#v, want kafka and localhost brokers", cfg.Kafka.Brokers)
	}
	if cfg.Auth.JWTSecret != "env-secret" {
		t.Fatalf("Auth.JWTSecret = %q, want env-secret", cfg.Auth.JWTSecret)
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
