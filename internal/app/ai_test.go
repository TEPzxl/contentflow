package app

import (
	"testing"

	"github.com/tepzxl/contentflow/internal/config"
	aimodule "github.com/tepzxl/contentflow/internal/module/ai"
)

func TestNewConfiguredAssistantDefaultsToLocal(t *testing.T) {
	assistant, err := newConfiguredAssistant(config.AIConfig{}, nil)
	if err != nil {
		t.Fatalf("newConfiguredAssistant() error = %v", err)
	}
	if _, ok := assistant.(*aimodule.ExtractiveAssistant); !ok {
		t.Fatalf("assistant type = %T, want *ai.ExtractiveAssistant", assistant)
	}
}

func TestNewConfiguredAssistantUsesOpenAIWhenAPIKeyIsConfigured(t *testing.T) {
	assistant, err := newConfiguredAssistant(config.AIConfig{
		Provider:       "local",
		BaseURL:        "http://ai.local/v1",
		APIKey:         "test-key",
		Model:          "chat-model",
		EmbeddingModel: "embed-model",
	}, nil)
	if err != nil {
		t.Fatalf("newConfiguredAssistant() error = %v", err)
	}
	if _, ok := assistant.(*aimodule.OpenAICompatibleAssistant); !ok {
		t.Fatalf("assistant type = %T, want *ai.OpenAICompatibleAssistant", assistant)
	}
}

func TestNewConfiguredAssistantRejectsIncompleteOpenAIConfig(t *testing.T) {
	_, err := newConfiguredAssistant(config.AIConfig{
		Provider: "openai",
		APIKey:   "test-key",
	}, nil)
	if err == nil {
		t.Fatal("newConfiguredAssistant() error is nil")
	}
}

func TestNewConfiguredAssistantRejectsUnknownProvider(t *testing.T) {
	_, err := newConfiguredAssistant(config.AIConfig{Provider: "unknown"}, nil)
	if err == nil {
		t.Fatal("newConfiguredAssistant() error is nil")
	}
}

func TestNewAISecretBoxAllowsEmptyKey(t *testing.T) {
	box, err := newAISecretBox("")
	if err != nil {
		t.Fatalf("newAISecretBox() error = %v", err)
	}
	if box != nil {
		t.Fatalf("box = %#v, want nil", box)
	}
}

func TestNewAISecretBoxRejectsInvalidKey(t *testing.T) {
	_, err := newAISecretBox("short")
	if err == nil {
		t.Fatal("newAISecretBox() error is nil")
	}
}
