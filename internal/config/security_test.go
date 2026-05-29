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

func TestLoadDockerConfigForLocalCompose(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.docker.yaml"))
	if err != nil {
		t.Fatalf("Load(config.docker.yaml) error = %v", err)
	}
	if cfg.App.Env != "dev" {
		t.Fatalf("App.Env = %q, want dev", cfg.App.Env)
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

func TestLoadAppliesLocalDotEnvAIAliases(t *testing.T) {
	unsetEnv(t, "API_KEY", "base_url", "model")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_KEY=test-key\nbase_url=http://ai.local/v1\nmodel=test-chat\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

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

	if cfg.AI.APIKey != "test-key" {
		t.Fatalf("AI.APIKey = %q, want test-key", cfg.AI.APIKey)
	}
	if cfg.AI.BaseURL != "http://ai.local/v1" {
		t.Fatalf("AI.BaseURL = %q, want http://ai.local/v1", cfg.AI.BaseURL)
	}
	if cfg.AI.Model != "test-chat" {
		t.Fatalf("AI.Model = %q, want test-chat", cfg.AI.Model)
	}
}

func TestLoadAppliesAISettingsEncryptionKey(t *testing.T) {
	t.Setenv("CONTENTFLOW_AI_SETTINGS_ENCRYPTION_KEY", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=")
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

	if cfg.AI.SettingsEncryptionKey == "" {
		t.Fatal("AI.SettingsEncryptionKey is empty")
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

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	original := make(map[string]*string, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			copied := value
			original[key] = &copied
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if value, ok := original[key]; ok {
				_ = os.Setenv(key, *value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	})
}
