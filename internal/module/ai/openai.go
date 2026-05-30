package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tepzxl/contentflow/internal/netguard"
)

const (
	DefaultOpenAIBaseURL        = "https://api.openai.com/v1"
	DefaultOpenAIEmbeddingModel = "text-embedding-3-small"
)

type OpenAIConfig struct {
	BaseURL        string
	APIKey         string
	ChatModel      string
	EmbeddingModel string
	Timeout        time.Duration
	HTTPClient     *http.Client
}

type OpenAICompatibleAssistant struct {
	baseURL        string
	apiKey         string
	chatModel      string
	embeddingModel string
	client         *http.Client
}

func NewOpenAICompatibleAssistant(cfg OpenAIConfig) (*OpenAICompatibleAssistant, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("openai api key is required")
	}
	chatModel := strings.TrimSpace(cfg.ChatModel)
	if chatModel == "" {
		return nil, errors.New("openai chat model is required")
	}
	embeddingModel := strings.TrimSpace(cfg.EmbeddingModel)
	if embeddingModel == "" {
		embeddingModel = DefaultOpenAIEmbeddingModel
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}
	if err := netguard.ValidateHTTPURL(baseURL); err != nil {
		return nil, fmt.Errorf("invalid openai base url: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout:       timeout,
			Transport:     safeOpenAITransport(),
			CheckRedirect: safeOpenAIRedirect,
		}
	}
	return &OpenAICompatibleAssistant{
		baseURL:        baseURL,
		apiKey:         apiKey,
		chatModel:      chatModel,
		embeddingModel: embeddingModel,
		client:         client,
	}, nil
}

func (a *OpenAICompatibleAssistant) Summarize(ctx context.Context, article ArticleInput) (SummaryResult, error) {
	content := strings.TrimSpace(firstNonEmpty(article.Content, article.Summary, article.Title))
	prompt := fmt.Sprintf("请用中文总结下面这篇文章，保留关键事实，控制在 3 句话以内。\n\n标题：%s\n\n内容：%s", article.Title, truncateText(content, 6000))
	text, err := a.chat(ctx, "你是内容聚合系统的文章摘要助手。", prompt)
	if err != nil {
		return SummaryResult{}, err
	}
	return SummaryResult{Model: a.chatModel, PromptVersion: DefaultSummaryPrompt, Summary: text}, nil
}

func (a *OpenAICompatibleAssistant) Embed(ctx context.Context, text string) (EmbeddingResult, error) {
	payload := embeddingRequest{
		Model: a.embeddingModel,
		Input: strings.TrimSpace(text),
	}
	var result embeddingResponse
	if err := a.post(ctx, "/embeddings", payload, &result); err != nil {
		return EmbeddingResult{}, err
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return EmbeddingResult{}, errors.New("openai embedding response is empty")
	}
	vector := result.Data[0].Embedding
	return EmbeddingResult{
		Model:      firstNonEmpty(result.Model, a.embeddingModel),
		Version:    DefaultEmbeddingVersion,
		Dimensions: len(vector),
		Vector:     vector,
	}, nil
}

func (a *OpenAICompatibleAssistant) Digest(ctx context.Context, articles []ArticleInput) (DigestResult, error) {
	if len(articles) == 0 {
		return DigestResult{
			Model:         a.chatModel,
			PromptVersion: DefaultDigestPrompt,
			Summary:       "今天没有可汇总的文章。",
			ArticleIDs:    []int64{},
		}, nil
	}
	var builder strings.Builder
	ids := make([]int64, 0, len(articles))
	for i, article := range articles {
		if i >= 10 {
			break
		}
		ids = append(ids, article.ID)
		fmt.Fprintf(&builder, "[%d] %s\n%s\n\n", article.ID, article.Title, truncateText(firstNonEmpty(article.Summary, article.Content), 800))
	}
	text, err := a.chat(ctx, "你是内容聚合系统的每日摘要助手。", "请根据下面的文章列表生成中文 Daily Digest，突出主题、重要变化和可行动信息。\n\n"+builder.String())
	if err != nil {
		return DigestResult{}, err
	}
	return DigestResult{Model: a.chatModel, PromptVersion: DefaultDigestPrompt, Summary: text, ArticleIDs: ids}, nil
}

func (a *OpenAICompatibleAssistant) Answer(ctx context.Context, query string, articles []ArticleInput) (RAGResult, error) {
	citations := make([]CitationDTO, 0, len(articles))
	var builder strings.Builder
	for _, article := range articles {
		snippet := truncateText(firstNonEmpty(article.Summary, article.Content, article.Title), 500)
		citations = append(citations, CitationDTO{ArticleID: article.ID, Title: article.Title, URL: article.URL, Snippet: snippet})
		fmt.Fprintf(&builder, "[%d] %s\n%s\n\n", article.ID, article.Title, snippet)
	}
	if len(citations) == 0 {
		return RAGResult{Model: a.chatModel, PromptVersion: DefaultRAGPrompt, Answer: "没有找到可引用的相关文章。", Citations: citations}, nil
	}

	prompt := fmt.Sprintf("问题：%s\n\n可引用文章：\n%s\n请只根据上面的文章回答，使用中文，无法从文章得出时明确说明。", strings.TrimSpace(query), builder.String())
	text, err := a.chat(ctx, "你是内容聚合系统的 RAG 问答助手。回答必须基于给定文章。", prompt)
	if err != nil {
		return RAGResult{}, err
	}
	return RAGResult{Model: a.chatModel, PromptVersion: DefaultRAGPrompt, Answer: text, Citations: citations}, nil
}

func (a *OpenAICompatibleAssistant) chat(ctx context.Context, system, user string) (string, error) {
	payload := chatCompletionRequest{
		Model: a.chatModel,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
	}
	var result chatCompletionResponse
	if err := a.post(ctx, "/chat/completions", payload, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("openai chat completion response is empty")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("openai chat completion content is empty")
	}
	return content, nil
}

func safeOpenAITransport() http.RoundTripper {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	clone := transport.Clone()
	clone.DialContext = netguard.DialContext
	return clone
}

func safeOpenAIRedirect(req *http.Request, via []*http.Request) error {
	if err := netguard.ValidateHTTPURL(req.URL.String()); err != nil {
		return err
	}
	if len(via) >= 10 {
		return http.ErrUseLastResponse
	}
	return nil
}

func (a *OpenAICompatibleAssistant) post(ctx context.Context, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal openai request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("call openai api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp openAIErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error.Message != "" {
			return fmt.Errorf("openai api status %d: %s", resp.StatusCode, errResp.Error.Message)
		}
		return fmt.Errorf("openai api status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode openai response: %w", err)
	}
	return nil
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
