package config_test

import (
	"testing"

	"github.com/tepzxl/contentflow/internal/config"
)

func TestPathFromEnv_UsesContentflowConfigWhenSet(t *testing.T) {
	t.Setenv("CONTENTFLOW_CONFIG", "/app/configs/config.docker.yaml")

	got := config.PathFromEnv("configs/config.yaml")

	if got != "/app/configs/config.docker.yaml" {
		t.Fatalf("PathFromEnv() = %q, want docker config path", got)
	}
}

func TestPathFromEnv_UsesDefaultWhenEnvEmpty(t *testing.T) {
	t.Setenv("CONTENTFLOW_CONFIG", "  ")

	got := config.PathFromEnv("configs/config.yaml")

	if got != "configs/config.yaml" {
		t.Fatalf("PathFromEnv() = %q, want default config path", got)
	}
}
