package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestOpenAICompatibleAssistant_EmbedCallsEmbeddingsAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %s, want /v1/embeddings", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want bearer key", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["model"] != "embed-model" || req["input"] != "hello world" {
			t.Fatalf("request = %#v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer server.Close()

	assistant, err := NewOpenAICompatibleAssistant(OpenAIConfig{
		BaseURL:        "https://api.openai.test/v1",
		APIKey:         "test-key",
		ChatModel:      "chat-model",
		EmbeddingModel: "embed-model",
		Timeout:        time.Second,
		HTTPClient:     newRewriteClient(t, server.URL),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleAssistant() error = %v", err)
	}

	result, err := assistant.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if result.Model != "embed-model" || result.Version != DefaultEmbeddingVersion || result.Dimensions != 3 {
		t.Fatalf("result metadata = %#v", result)
	}
	if len(result.Vector) != 3 || result.Vector[2] != 0.3 {
		t.Fatalf("vector = %#v", result.Vector)
	}
}

func TestOpenAICompatibleAssistant_RejectsUnsafeBaseURL(t *testing.T) {
	_, err := NewOpenAICompatibleAssistant(OpenAIConfig{
		BaseURL:        "http://127.0.0.1:8080/v1",
		APIKey:         "test-key",
		ChatModel:      "chat-model",
		EmbeddingModel: "embed-model",
	})
	if err == nil {
		t.Fatal("NewOpenAICompatibleAssistant() error = nil, want unsafe base url error")
	}
}

func TestOpenAICompatibleAssistant_AnswerCallsChatCompletionsAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want bearer key", got)
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "chat-model" || len(req.Messages) != 2 {
			t.Fatalf("request = %#v", req)
		}
		if req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			t.Fatalf("messages = %#v", req.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"chat-model","choices":[{"message":{"content":"Use the cited article."}}]}`))
	}))
	defer server.Close()

	assistant, err := NewOpenAICompatibleAssistant(OpenAIConfig{
		BaseURL:        "https://api.openai.test/v1",
		APIKey:         "test-key",
		ChatModel:      "chat-model",
		EmbeddingModel: "embed-model",
		Timeout:        time.Second,
		HTTPClient:     newRewriteClient(t, server.URL),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleAssistant() error = %v", err)
	}

	result, err := assistant.Answer(context.Background(), "what matters?", []ArticleInput{
		{ID: 7, Title: "Reliability", Summary: "Retries matter.", Content: "DLQ preserves failed jobs."},
	})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if result.Model != "chat-model" || result.PromptVersion != DefaultRAGPrompt || result.Answer != "Use the cited article." {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Citations) != 1 || result.Citations[0].ArticleID != 7 {
		t.Fatalf("citations = %#v", result.Citations)
	}
}

func newRewriteClient(t *testing.T, target string) *http.Client {
	t.Helper()
	targetURL, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse target server url: %v", err)
	}
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			clone := req.Clone(req.Context())
			clone.URL.Scheme = targetURL.Scheme
			clone.URL.Host = targetURL.Host
			clone.Host = targetURL.Host
			return http.DefaultTransport.RoundTrip(clone)
		}),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
