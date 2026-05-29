package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/article"
	"github.com/tepzxl/contentflow/internal/secrets"
)

func TestService_RequestSummaryQueuesPendingJob(t *testing.T) {
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "Go reliability", Content: "Retries and DLQ matter."}

	service := NewService(repo, articles, fakeAssistant{}, WithNow(func() time.Time { return now }))
	result, err := service.RequestSummary(context.Background(), RequestSummaryRequest{
		UserID:     10,
		ArticleID:  1,
		Regenerate: true,
	})
	if err != nil {
		t.Fatalf("RequestSummary() error = %v", err)
	}
	if result.Status != StatusPending || result.PromptVersion != DefaultSummaryPrompt {
		t.Fatalf("summary = %#v", result)
	}
	if repo.enqueued == nil || !repo.enqueued.Regenerate {
		t.Fatalf("enqueued = %#v", repo.enqueued)
	}
}

func TestService_RequestSummaryUsesSucceededCache(t *testing.T) {
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.summary = &ArticleSummary{ID: 4, UserID: 10, ArticleID: 1, Model: DefaultSummaryModel, PromptVersion: DefaultSummaryPrompt, Status: StatusSucceeded, Summary: "cached"}
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "Go reliability", Content: "Retries and DLQ matter."}

	service := NewService(repo, articles, fakeAssistant{}, WithNow(func() time.Time { return now }))
	result, err := service.RequestSummary(context.Background(), RequestSummaryRequest{
		UserID:    10,
		ArticleID: 1,
	})
	if err != nil {
		t.Fatalf("RequestSummary() error = %v", err)
	}
	if result.Summary != "cached" {
		t.Fatalf("summary = %q, want cached", result.Summary)
	}
	if repo.enqueued != nil {
		t.Fatalf("enqueued = %#v, want nil", repo.enqueued)
	}
}

func TestService_ProcessNextSummaryCompletesWithPromptMetadata(t *testing.T) {
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.nextSummary = &ArticleSummary{ID: 7, UserID: 10, ArticleID: 1, Model: DefaultSummaryModel, PromptVersion: DefaultSummaryPrompt}
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "AI summary", Content: "First sentence. Second sentence."}

	service := NewService(repo, articles, fakeAssistant{}, WithNow(func() time.Time { return now }))
	processed, err := service.ProcessNextSummary(context.Background())
	if err != nil {
		t.Fatalf("ProcessNextSummary() error = %v", err)
	}
	if !processed {
		t.Fatal("processed = false, want true")
	}
	if repo.completedID != 7 || repo.completedSummary != "generated summary" {
		t.Fatalf("completed id/summary = %d/%q", repo.completedID, repo.completedSummary)
	}
}

func TestService_ProcessNextSummaryMarksRetryableFailure(t *testing.T) {
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.nextSummary = &ArticleSummary{ID: 9, UserID: 10, ArticleID: 1, Attempts: 1}
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "AI summary", Content: "Content"}

	service := NewService(repo, articles, fakeAssistant{summarizeErr: errors.New("llm timeout")}, WithNow(func() time.Time { return now }), WithRetry(3, time.Minute))
	processed, err := service.ProcessNextSummary(context.Background())
	if err == nil {
		t.Fatal("ProcessNextSummary() error = nil, want error")
	}
	if !processed {
		t.Fatal("processed = false, want true")
	}
	if repo.failedID != 9 || repo.failedMessage != "llm timeout" {
		t.Fatalf("failed id/message = %d/%q", repo.failedID, repo.failedMessage)
	}
	if !repo.failedNextAttempt.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("next attempt = %s, want %s", repo.failedNextAttempt, now.Add(2*time.Minute))
	}
}

func TestService_SimilarArticlesUsesUserScopedEmbeddings(t *testing.T) {
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.embeddings = []EmbeddingRecord{
		{ID: 1, UserID: 10, ArticleID: 1, Model: DefaultEmbeddingModel, Version: DefaultEmbeddingVersion, Embedding: []float64{1, 0}},
		{ID: 2, UserID: 10, ArticleID: 2, Model: DefaultEmbeddingModel, Version: DefaultEmbeddingVersion, Embedding: []float64{0.9, 0.1}},
		{ID: 3, UserID: 11, ArticleID: 3, Model: DefaultEmbeddingModel, Version: DefaultEmbeddingVersion, Embedding: []float64{1, 0}},
	}
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "One"}
	articles.rows[2] = article.ArticleWithState{ID: 2, Title: "Two", Summary: "similar"}
	articles.rows[3] = article.ArticleWithState{ID: 3, Title: "Other user"}

	service := NewService(repo, articles, fakeAssistant{}, WithNow(func() time.Time { return now }))
	result, err := service.SimilarArticles(context.Background(), SimilarArticlesRequest{UserID: 10, ArticleID: 1, Limit: 5})
	if err != nil {
		t.Fatalf("SimilarArticles() error = %v", err)
	}
	if len(result) != 1 || result[0].ArticleID != 2 {
		t.Fatalf("similar result = %#v", result)
	}
}

func TestService_SimilarArticlesUsesGeneratedEmbeddingModel(t *testing.T) {
	repo := newFakeRepository()
	repo.embeddings = []EmbeddingRecord{
		{ID: 2, UserID: 10, ArticleID: 2, Model: "fake-embedding", Version: DefaultEmbeddingVersion, Embedding: []float64{1, 0}},
	}
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "Target", Content: "Semantic target"}
	articles.rows[2] = article.ArticleWithState{ID: 2, Title: "Generated model hit", Summary: "Semantic neighbor"}

	service := NewService(repo, articles, fakeAssistant{embedding: []float64{1, 0}})
	result, err := service.SimilarArticles(context.Background(), SimilarArticlesRequest{UserID: 10, ArticleID: 1, Limit: 5})
	if err != nil {
		t.Fatalf("SimilarArticles() error = %v", err)
	}
	if len(result) != 1 || result[0].ArticleID != 2 {
		t.Fatalf("similar result = %#v, want article 2 from generated embedding model", result)
	}
}

func TestService_RAGSearchReturnsCitations(t *testing.T) {
	repo := newFakeRepository()
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "Kafka retry", Summary: "Retry failed jobs", Content: "DLQ preserves failed jobs."}

	service := NewService(repo, articles, fakeAssistant{})
	result, err := service.RAGSearch(context.Background(), RAGSearchRequest{UserID: 10, Query: "retry", Limit: 3})
	if err != nil {
		t.Fatalf("RAGSearch() error = %v", err)
	}
	if result.Answer == "" || len(result.Citations) != 1 || result.Citations[0].ArticleID != 1 {
		t.Fatalf("rag result = %#v", result)
	}
}

func TestService_RAGSearchUsesEmbeddingRetrievalWhenAvailable(t *testing.T) {
	repo := newFakeRepository()
	repo.embeddings = []EmbeddingRecord{
		{ID: 1, UserID: 10, ArticleID: 1, Model: "fake-embedding", Version: "embedding-v1", Embedding: []float64{0, 1}},
		{ID: 2, UserID: 10, ArticleID: 2, Model: "fake-embedding", Version: "embedding-v1", Embedding: []float64{1, 0}},
	}
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "Unrelated", Summary: "Other topic"}
	articles.rows[2] = article.ArticleWithState{ID: 2, Title: "Vector hit", Summary: "Semantic answer"}

	service := NewService(repo, articles, fakeAssistant{embedding: []float64{1, 0}})
	result, err := service.RAGSearch(context.Background(), RAGSearchRequest{UserID: 10, Query: "semantic", Limit: 1})
	if err != nil {
		t.Fatalf("RAGSearch() error = %v", err)
	}
	if len(result.Citations) != 1 || result.Citations[0].ArticleID != 2 {
		t.Fatalf("citations = %#v, want article 2 from embedding retrieval", result.Citations)
	}
}

func TestService_RAGSearchUsesUserAISettingsAssistant(t *testing.T) {
	repo := newFakeRepository()
	box := testSecretBox(t)
	ciphertext, nonce, err := box.EncryptString("sk-user")
	if err != nil {
		t.Fatalf("encrypt api key: %v", err)
	}
	repo.aiSettings = &UserAISettingsRecord{
		UserID:           10,
		Provider:         "openai-compatible",
		BaseURL:          "https://example.com/v1",
		Model:            "chat-model",
		EmbeddingModel:   "embed-model",
		APIKeyCiphertext: ciphertext,
		APIKeyNonce:      nonce,
	}
	repo.embeddings = []EmbeddingRecord{
		{ID: 1, UserID: 10, ArticleID: 1, Model: "embed-model", Version: DefaultEmbeddingVersion, Embedding: []float64{1, 0}},
	}
	articles := newFakeArticleRepository()
	articles.rows[1] = article.ArticleWithState{ID: 1, Title: "User setting hit", Summary: "Configured assistant result"}
	var gotConfig OpenAIConfig
	service := NewService(
		repo,
		articles,
		fakeAssistant{embeddingModel: "fallback-model"},
		WithSecretBox(box),
		WithAssistantFactory(func(cfg OpenAIConfig) (Assistant, error) {
			gotConfig = cfg
			return fakeAssistant{embeddingModel: cfg.EmbeddingModel, answerModel: cfg.ChatModel, embedding: []float64{1, 0}}, nil
		}),
	)

	result, err := service.RAGSearch(context.Background(), RAGSearchRequest{UserID: 10, Query: "semantic", Limit: 1})
	if err != nil {
		t.Fatalf("RAGSearch() error = %v", err)
	}

	if gotConfig.APIKey != "sk-user" || gotConfig.ChatModel != "chat-model" || gotConfig.EmbeddingModel != "embed-model" {
		t.Fatalf("assistant config = %#v", gotConfig)
	}
	if result.Model != "chat-model" || len(result.Citations) != 1 || result.Citations[0].ArticleID != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestService_UpdateAISettingsEncryptsAPIKeyAndRedactsResponse(t *testing.T) {
	repo := newFakeRepository()
	box := testSecretBox(t)
	service := NewService(repo, newFakeArticleRepository(), fakeAssistant{}, WithSecretBox(box))
	apiKey := "sk-test"

	result, err := service.UpdateAISettings(context.Background(), UpdateAISettingsRequest{
		UserID:         10,
		Provider:       "openai-compatible",
		BaseURL:        "https://example.com/v1",
		Model:          "chat-model",
		EmbeddingModel: "embed-model",
		APIKey:         &apiKey,
	})
	if err != nil {
		t.Fatalf("UpdateAISettings() error = %v", err)
	}

	if !result.HasAPIKey {
		t.Fatal("HasAPIKey = false, want true")
	}
	if result.Provider != "openai-compatible" || result.Model != "chat-model" {
		t.Fatalf("result = %#v", result)
	}
	if string(repo.aiSettings.APIKeyCiphertext) == apiKey {
		t.Fatal("stored api key ciphertext is plaintext")
	}
	if len(repo.aiSettings.APIKeyCiphertext) == 0 || len(repo.aiSettings.APIKeyNonce) == 0 {
		t.Fatalf("encrypted fields are empty: %#v", repo.aiSettings)
	}
}

func TestService_UpdateAISettingsPreservesExistingAPIKeyWhenOmitted(t *testing.T) {
	repo := newFakeRepository()
	box := testSecretBox(t)
	ciphertext, nonce, err := box.EncryptString("sk-existing")
	if err != nil {
		t.Fatalf("encrypt api key: %v", err)
	}
	repo.aiSettings = &UserAISettingsRecord{
		UserID:           10,
		Provider:         "openai-compatible",
		BaseURL:          "https://example.com/v1",
		Model:            "old-chat-model",
		EmbeddingModel:   "old-embed-model",
		APIKeyCiphertext: ciphertext,
		APIKeyNonce:      nonce,
	}
	service := NewService(repo, newFakeArticleRepository(), fakeAssistant{}, WithSecretBox(box))

	result, err := service.UpdateAISettings(context.Background(), UpdateAISettingsRequest{
		UserID:         10,
		Provider:       "openai-compatible",
		BaseURL:        "https://example.com/v2",
		Model:          "new-chat-model",
		EmbeddingModel: "new-embed-model",
	})
	if err != nil {
		t.Fatalf("UpdateAISettings() error = %v", err)
	}

	if !result.HasAPIKey {
		t.Fatal("HasAPIKey = false, want true")
	}
	if string(repo.aiSettings.APIKeyCiphertext) != string(ciphertext) || string(repo.aiSettings.APIKeyNonce) != string(nonce) {
		t.Fatal("stored api key changed when request omitted api_key")
	}
	if result.BaseURL != "https://example.com/v2" || result.Model != "new-chat-model" {
		t.Fatalf("result = %#v", result)
	}
}

func TestService_UpdateAISettingsClearsAPIKeyWhenEmptyString(t *testing.T) {
	repo := newFakeRepository()
	box := testSecretBox(t)
	ciphertext, nonce, err := box.EncryptString("sk-existing")
	if err != nil {
		t.Fatalf("encrypt api key: %v", err)
	}
	repo.aiSettings = &UserAISettingsRecord{
		UserID:           10,
		Provider:         "openai-compatible",
		BaseURL:          "https://example.com/v1",
		Model:            "chat-model",
		EmbeddingModel:   "embed-model",
		APIKeyCiphertext: ciphertext,
		APIKeyNonce:      nonce,
	}
	service := NewService(repo, newFakeArticleRepository(), fakeAssistant{}, WithSecretBox(box))
	apiKey := ""

	result, err := service.UpdateAISettings(context.Background(), UpdateAISettingsRequest{
		UserID:         10,
		Provider:       "openai-compatible",
		BaseURL:        "https://example.com/v1",
		Model:          "chat-model",
		EmbeddingModel: "embed-model",
		APIKey:         &apiKey,
	})
	if err != nil {
		t.Fatalf("UpdateAISettings() error = %v", err)
	}

	if result.HasAPIKey {
		t.Fatal("HasAPIKey = true, want false")
	}
	if len(repo.aiSettings.APIKeyCiphertext) != 0 || len(repo.aiSettings.APIKeyNonce) != 0 {
		t.Fatalf("stored api key fields = %d/%d, want empty", len(repo.aiSettings.APIKeyCiphertext), len(repo.aiSettings.APIKeyNonce))
	}
}

func TestService_UpdateAISettingsRejectsUnknownProvider(t *testing.T) {
	service := NewService(newFakeRepository(), newFakeArticleRepository(), fakeAssistant{})

	_, err := service.UpdateAISettings(context.Background(), UpdateAISettingsRequest{
		UserID:   10,
		Provider: "anthropic",
	})
	if !errors.Is(err, ErrInvalidAIProvider) {
		t.Fatalf("UpdateAISettings() error = %v, want ErrInvalidAIProvider", err)
	}
}

func TestService_UpdateAISettingsRejectsUnsafeBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "loopback", baseURL: "http://127.0.0.1:8080/v1"},
		{name: "localhost", baseURL: "http://localhost:8080/v1"},
		{name: "link local metadata", baseURL: "http://169.254.169.254/latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(newFakeRepository(), newFakeArticleRepository(), fakeAssistant{})

			_, err := service.UpdateAISettings(context.Background(), UpdateAISettingsRequest{
				UserID:   10,
				Provider: "openai-compatible",
				BaseURL:  tt.baseURL,
				Model:    "chat-model",
			})
			if !errors.Is(err, ErrInvalidAIBaseURL) {
				t.Fatalf("UpdateAISettings() error = %v, want ErrInvalidAIBaseURL", err)
			}
		})
	}
}

func TestService_GetAISettingsRedactsEncryptedAPIKey(t *testing.T) {
	repo := newFakeRepository()
	box := testSecretBox(t)
	ciphertext, nonce, err := box.EncryptString("sk-test")
	if err != nil {
		t.Fatalf("encrypt api key: %v", err)
	}
	repo.aiSettings = &UserAISettingsRecord{
		UserID:           10,
		Provider:         "openai-compatible",
		BaseURL:          "https://example.com/v1",
		Model:            "chat-model",
		EmbeddingModel:   "embed-model",
		APIKeyCiphertext: ciphertext,
		APIKeyNonce:      nonce,
	}
	service := NewService(repo, newFakeArticleRepository(), fakeAssistant{}, WithSecretBox(box))

	result, err := service.GetAISettings(context.Background(), GetAISettingsRequest{UserID: 10})
	if err != nil {
		t.Fatalf("GetAISettings() error = %v", err)
	}

	if !result.HasAPIKey {
		t.Fatal("HasAPIKey = false, want true")
	}
	if result.Provider != "openai-compatible" || result.BaseURL != "https://example.com/v1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestService_UpdateAISettingsRejectsAPIKeyWithoutEncryptionKey(t *testing.T) {
	service := NewService(newFakeRepository(), newFakeArticleRepository(), fakeAssistant{})
	apiKey := "sk-test"

	_, err := service.UpdateAISettings(context.Background(), UpdateAISettingsRequest{
		UserID:   10,
		Provider: "openai-compatible",
		BaseURL:  "https://example.com/v1",
		Model:    "chat-model",
		APIKey:   &apiKey,
	})
	if !errors.Is(err, ErrAISettingsEncryptionKeyRequired) {
		t.Fatalf("UpdateAISettings() error = %v, want ErrAISettingsEncryptionKeyRequired", err)
	}
}

type fakeAssistant struct {
	summarizeErr   error
	embedding      []float64
	embeddingModel string
	answerModel    string
}

func (a fakeAssistant) Summarize(context.Context, ArticleInput) (SummaryResult, error) {
	if a.summarizeErr != nil {
		return SummaryResult{}, a.summarizeErr
	}
	return SummaryResult{Model: DefaultSummaryModel, PromptVersion: DefaultSummaryPrompt, Summary: "generated summary"}, nil
}

func (a fakeAssistant) Embed(context.Context, string) (EmbeddingResult, error) {
	vector := a.embedding
	if vector == nil {
		vector = []float64{1, 0}
	}
	model := firstNonEmpty(a.embeddingModel, "fake-embedding")
	return EmbeddingResult{Model: model, Version: DefaultEmbeddingVersion, Dimensions: len(vector), Vector: vector}, nil
}

func (a fakeAssistant) Digest(context.Context, []ArticleInput) (DigestResult, error) {
	return DigestResult{Model: DefaultSummaryModel, PromptVersion: DefaultDigestPrompt, Summary: "digest", ArticleIDs: []int64{1}}, nil
}

func (a fakeAssistant) Answer(_ context.Context, _ string, articles []ArticleInput) (RAGResult, error) {
	citations := make([]CitationDTO, 0, len(articles))
	for _, item := range articles {
		citations = append(citations, CitationDTO{ArticleID: item.ID, Title: item.Title, URL: item.URL, Snippet: item.Summary})
	}
	return RAGResult{Model: firstNonEmpty(a.answerModel, DefaultSummaryModel), PromptVersion: DefaultRAGPrompt, Answer: "answer", Citations: citations}, nil
}

type fakeRepository struct {
	enqueued          *EnqueueSummaryParams
	nextSummary       *ArticleSummary
	completedID       int64
	completedSummary  string
	failedID          int64
	failedMessage     string
	failedNextAttempt time.Time
	summary           *ArticleSummary
	embeddings        []EmbeddingRecord
	aiSettings        *UserAISettingsRecord
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{}
}

func (r *fakeRepository) EnqueueSummary(_ context.Context, params EnqueueSummaryParams) (*ArticleSummary, error) {
	r.enqueued = &params
	return &ArticleSummary{ID: 1, UserID: params.UserID, ArticleID: params.ArticleID, Model: params.Model, PromptVersion: params.PromptVersion, Status: StatusPending, CreatedAt: params.Now, UpdatedAt: params.Now}, nil
}

func (r *fakeRepository) FindSummary(context.Context, int64, int64, string) (*ArticleSummary, error) {
	if r.summary != nil {
		return r.summary, nil
	}
	return nil, ErrSummaryNotFound
}

func (r *fakeRepository) ClaimNextSummary(context.Context, time.Time, int) (*ArticleSummary, error) {
	if r.nextSummary == nil {
		return nil, ErrNoSummaryJob
	}
	return r.nextSummary, nil
}

func (r *fakeRepository) CompleteSummary(_ context.Context, id int64, summary string, _ time.Time) (*ArticleSummary, error) {
	r.completedID = id
	r.completedSummary = summary
	return &ArticleSummary{ID: id, Summary: summary, Status: StatusSucceeded}, nil
}

func (r *fakeRepository) FailSummary(_ context.Context, id int64, errMessage string, nextAttemptAt time.Time, _ time.Time) (*ArticleSummary, error) {
	r.failedID = id
	r.failedMessage = errMessage
	r.failedNextAttempt = nextAttemptAt
	return &ArticleSummary{ID: id, ErrorMessage: errMessage, Status: StatusFailed}, nil
}

func (r *fakeRepository) UpsertEmbedding(_ context.Context, params UpsertEmbeddingParams) (*EmbeddingRecord, error) {
	record := EmbeddingRecord{ID: 1, UserID: params.UserID, ArticleID: params.ArticleID, Model: params.Model, Version: params.Version, Dimensions: params.Dimensions, Embedding: params.Embedding, ContentHash: params.ContentHash}
	r.embeddings = append(r.embeddings, record)
	return &record, nil
}

func (r *fakeRepository) FindEmbedding(_ context.Context, userID, articleID int64, model, version string) (*EmbeddingRecord, error) {
	for _, item := range r.embeddings {
		if item.UserID == userID && item.ArticleID == articleID && item.Model == model && item.Version == version {
			return &item, nil
		}
	}
	return nil, ErrEmbeddingNotFound
}

func (r *fakeRepository) ListEmbeddingsByUser(_ context.Context, userID int64, model, version string, _ int) ([]EmbeddingRecord, error) {
	result := []EmbeddingRecord{}
	for _, item := range r.embeddings {
		if item.UserID == userID && item.Model == model && item.Version == version {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r *fakeRepository) UpsertDigest(_ context.Context, params UpsertDigestParams) (*DigestRecord, error) {
	return &DigestRecord{ID: 1, UserID: params.UserID, DigestDate: params.DigestDate, Model: params.Model, PromptVersion: params.PromptVersion, Summary: params.Summary, ArticleIDs: params.ArticleIDs, Status: StatusSucceeded, CreatedAt: params.Now, UpdatedAt: params.Now}, nil
}

func (r *fakeRepository) FindDigest(context.Context, int64, time.Time, string) (*DigestRecord, error) {
	return nil, ErrDigestNotFound
}

func (r *fakeRepository) FindAISettings(_ context.Context, userID int64) (*UserAISettingsRecord, error) {
	if r.aiSettings == nil || r.aiSettings.UserID != userID {
		return nil, ErrAISettingsNotFound
	}
	return r.aiSettings, nil
}

func (r *fakeRepository) UpsertAISettings(_ context.Context, params UpsertAISettingsParams) (*UserAISettingsRecord, error) {
	record := UserAISettingsRecord{
		ID:               1,
		UserID:           params.UserID,
		Provider:         params.Provider,
		BaseURL:          params.BaseURL,
		Model:            params.Model,
		EmbeddingModel:   params.EmbeddingModel,
		APIKeyCiphertext: params.APIKeyCiphertext,
		APIKeyNonce:      params.APIKeyNonce,
		CreatedAt:        params.Now,
		UpdatedAt:        params.Now,
	}
	r.aiSettings = &record
	return &record, nil
}

func testSecretBox(t *testing.T) *secrets.AESGCM {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	box, err := secrets.NewAESGCMFromEncodedKey(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("NewAESGCMFromEncodedKey() error = %v", err)
	}
	return box
}

type fakeArticleRepository struct {
	rows map[int64]article.ArticleWithState
}

func newFakeArticleRepository() *fakeArticleRepository {
	return &fakeArticleRepository{rows: map[int64]article.ArticleWithState{}}
}

func (r *fakeArticleRepository) CreateIfNotExists(context.Context, *article.Article) (bool, error) {
	return false, nil
}

func (r *fakeArticleRepository) ListByUser(_ context.Context, params article.ListArticlesParams) ([]article.ArticleWithState, int64, error) {
	result := []article.ArticleWithState{}
	for _, row := range r.rows {
		if params.Query == "" || row.Title == params.Query || row.Summary != "" || row.Content != "" {
			result = append(result, row)
		}
	}
	return result, int64(len(result)), nil
}

func (r *fakeArticleRepository) FindByUserAndID(_ context.Context, _ int64, articleID int64) (article.ArticleWithState, error) {
	row, ok := r.rows[articleID]
	if !ok {
		return article.ArticleWithState{}, article.ErrArticleNotFound
	}
	return row, nil
}

func (r *fakeArticleRepository) UpsertState(context.Context, article.UpsertArticleStateParams) (article.ArticleWithState, error) {
	return article.ArticleWithState{}, nil
}
