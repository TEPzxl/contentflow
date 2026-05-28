package app

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/tepzxl/contentflow/internal/config"
	"github.com/tepzxl/contentflow/internal/module/ai"
)

func newConfiguredAssistant(cfg config.AIConfig, log *slog.Logger) (ai.Assistant, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "local"
	}
	if strings.TrimSpace(cfg.APIKey) != "" && provider == "local" {
		provider = "openai"
	}

	switch provider {
	case "local", "extractive":
		if log != nil {
			log.Info("ai assistant configured", slog.String("provider", "local"))
		}
		return ai.NewExtractiveAssistant(), nil
	case "openai", "openai-compatible":
		assistant, err := ai.NewOpenAICompatibleAssistant(ai.OpenAIConfig{
			BaseURL:        cfg.BaseURL,
			APIKey:         cfg.APIKey,
			ChatModel:      cfg.Model,
			EmbeddingModel: cfg.EmbeddingModel,
			Timeout:        cfg.Timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("init openai-compatible ai assistant: %w", err)
		}
		if log != nil {
			log.Info("ai assistant configured",
				slog.String("provider", "openai-compatible"),
				slog.String("base_url", cfg.BaseURL),
				slog.String("model", cfg.Model),
				slog.String("embedding_model", cfg.EmbeddingModel),
			)
		}
		return assistant, nil
	default:
		return nil, fmt.Errorf("unsupported ai provider %q", cfg.Provider)
	}
}
