package ai

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tepzxl/contentflow/internal/module/article"
	"github.com/tepzxl/contentflow/internal/netguard"
)

type Service struct {
	repo             Repository
	articles         article.Repository
	assistant        Assistant
	assistantFactory AssistantFactory
	secretBox        SecretBox
	now              func() time.Time
	maxAttempts      int
	retryBackoff     time.Duration
}

type AssistantFactory func(cfg OpenAIConfig) (Assistant, error)

type SecretBox interface {
	EncryptString(plaintext string) ([]byte, []byte, error)
	DecryptString(ciphertext, nonce []byte) (string, error)
}

type Option func(*Service)

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
	}
}

func WithRetry(maxAttempts int, backoff time.Duration) Option {
	return func(s *Service) {
		if maxAttempts > 0 {
			s.maxAttempts = maxAttempts
		}
		if backoff > 0 {
			s.retryBackoff = backoff
		}
	}
}

func WithSecretBox(box SecretBox) Option {
	return func(s *Service) {
		s.secretBox = box
	}
}

func WithAssistantFactory(factory AssistantFactory) Option {
	return func(s *Service) {
		if factory != nil {
			s.assistantFactory = factory
		}
	}
}

func NewService(repo Repository, articles article.Repository, assistant Assistant, opts ...Option) *Service {
	if assistant == nil {
		assistant = NewExtractiveAssistant()
	}
	s := &Service{
		repo:             repo,
		articles:         articles,
		assistant:        assistant,
		assistantFactory: defaultAssistantFactory,
		now:              time.Now,
		maxAttempts:      3,
		retryBackoff:     time.Minute,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) RequestSummary(ctx context.Context, req RequestSummaryRequest) (*SummaryDTO, error) {
	if req.UserID <= 0 || req.ArticleID <= 0 {
		return nil, article.ErrArticleNotFound
	}
	if _, err := s.articles.FindByUserAndID(ctx, req.UserID, req.ArticleID); err != nil {
		return nil, err
	}
	if !req.Regenerate {
		existing, err := s.repo.FindSummary(ctx, req.UserID, req.ArticleID, DefaultSummaryPrompt)
		if err == nil && existing.Status == StatusSucceeded {
			dto := summaryToDTO(*existing)
			return &dto, nil
		}
		if err != nil && !errors.Is(err, ErrSummaryNotFound) {
			return nil, err
		}
	}

	summary, err := s.repo.EnqueueSummary(ctx, EnqueueSummaryParams{
		UserID:        req.UserID,
		ArticleID:     req.ArticleID,
		Model:         DefaultSummaryModel,
		PromptVersion: DefaultSummaryPrompt,
		Now:           s.now(),
		Regenerate:    req.Regenerate,
	})
	if err != nil {
		return nil, fmt.Errorf("request article summary: %w", err)
	}
	dto := summaryToDTO(*summary)
	return &dto, nil
}

func (s *Service) GetSummary(ctx context.Context, req GetSummaryRequest) (*SummaryDTO, error) {
	summary, err := s.repo.FindSummary(ctx, req.UserID, req.ArticleID, DefaultSummaryPrompt)
	if err != nil {
		return nil, err
	}
	dto := summaryToDTO(*summary)
	return &dto, nil
}

func (s *Service) GetAISettings(ctx context.Context, req GetAISettingsRequest) (*AISettingsDTO, error) {
	record, err := s.repo.FindAISettings(ctx, req.UserID)
	if errors.Is(err, ErrAISettingsNotFound) {
		dto := defaultAISettingsDTO()
		return &dto, nil
	}
	if err != nil {
		return nil, err
	}
	dto := aiSettingsToDTO(*record)
	return &dto, nil
}

func (s *Service) UpdateAISettings(ctx context.Context, req UpdateAISettingsRequest) (*AISettingsDTO, error) {
	provider, err := normalizeAIProvider(req.Provider)
	if err != nil {
		return nil, err
	}

	baseURL, err := normalizeAIBaseURL(req.BaseURL)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(req.Model)
	if provider == "openai-compatible" && model == "" {
		return nil, ErrInvalidAIModel
	}

	current, err := s.repo.FindAISettings(ctx, req.UserID)
	if err != nil && !errors.Is(err, ErrAISettingsNotFound) {
		return nil, err
	}

	ciphertext := []byte(nil)
	nonce := []byte(nil)
	if current != nil {
		ciphertext = current.APIKeyCiphertext
		nonce = current.APIKeyNonce
	}

	if req.APIKey != nil {
		apiKey := strings.TrimSpace(*req.APIKey)
		if apiKey == "" {
			ciphertext = nil
			nonce = nil
		} else {
			if s.secretBox == nil {
				return nil, ErrAISettingsEncryptionKeyRequired
			}
			ciphertext, nonce, err = s.secretBox.EncryptString(apiKey)
			if err != nil {
				return nil, fmt.Errorf("encrypt ai api key: %w", err)
			}
		}
	}

	record, err := s.repo.UpsertAISettings(ctx, UpsertAISettingsParams{
		UserID:           req.UserID,
		Provider:         provider,
		BaseURL:          baseURL,
		Model:            model,
		EmbeddingModel:   firstNonEmpty(req.EmbeddingModel, DefaultOpenAIEmbeddingModel),
		APIKeyCiphertext: ciphertext,
		APIKeyNonce:      nonce,
		Now:              s.now(),
	})
	if err != nil {
		return nil, err
	}
	dto := aiSettingsToDTO(*record)
	return &dto, nil
}

func (s *Service) ProcessNextSummary(ctx context.Context) (bool, error) {
	job, err := s.repo.ClaimNextSummary(ctx, s.now(), s.maxAttempts)
	if errors.Is(err, ErrNoSummaryJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	row, err := s.articles.FindByUserAndID(ctx, job.UserID, job.ArticleID)
	if err != nil {
		if _, failErr := s.repo.FailSummary(ctx, job.ID, err.Error(), s.nextAttempt(job.Attempts), s.now()); failErr != nil {
			return true, failErr
		}
		return true, err
	}

	assistant, err := s.assistantForUser(ctx, job.UserID)
	if err != nil {
		if _, failErr := s.repo.FailSummary(ctx, job.ID, err.Error(), s.nextAttempt(job.Attempts), s.now()); failErr != nil {
			return true, failErr
		}
		return true, err
	}

	result, err := assistant.Summarize(ctx, articleInput(row))
	if err != nil {
		if _, failErr := s.repo.FailSummary(ctx, job.ID, err.Error(), s.nextAttempt(job.Attempts), s.now()); failErr != nil {
			return true, failErr
		}
		return true, err
	}
	if _, err := s.repo.CompleteSummary(ctx, job.ID, result.Summary, s.now()); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Service) GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*EmbeddingDTO, error) {
	row, err := s.articles.FindByUserAndID(ctx, req.UserID, req.ArticleID)
	if err != nil {
		return nil, err
	}

	assistant, err := s.assistantForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	result, err := assistant.Embed(ctx, embeddingText(row))
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}
	record, err := s.repo.UpsertEmbedding(ctx, UpsertEmbeddingParams{
		UserID:      req.UserID,
		ArticleID:   req.ArticleID,
		Model:       result.Model,
		Version:     result.Version,
		Dimensions:  result.Dimensions,
		Embedding:   result.Vector,
		ContentHash: contentHash(row),
		Now:         s.now(),
	})
	if err != nil {
		return nil, err
	}
	dto := embeddingToDTO(*record)
	return &dto, nil
}

func (s *Service) SimilarArticles(ctx context.Context, req SimilarArticlesRequest) ([]SimilarArticleDTO, error) {
	limit := normalizeLimit(req.Limit, 5, 20)
	target, err := s.repo.FindEmbedding(ctx, req.UserID, req.ArticleID, DefaultEmbeddingModel, DefaultEmbeddingVersion)
	if errors.Is(err, ErrEmbeddingNotFound) {
		generated, genErr := s.GenerateEmbedding(ctx, GenerateEmbeddingRequest{UserID: req.UserID, ArticleID: req.ArticleID})
		if genErr != nil {
			return nil, genErr
		}
		target, err = s.repo.FindEmbedding(ctx, req.UserID, req.ArticleID, generated.Model, generated.Version)
	}
	if err != nil {
		return nil, err
	}

	records, err := s.repo.ListEmbeddingsByUser(ctx, req.UserID, target.Model, target.Version, 500)
	if err != nil {
		return nil, err
	}

	scored := make([]similarScore, 0, len(records))
	for _, record := range records {
		if record.ArticleID == req.ArticleID {
			continue
		}
		score := cosineSimilarity(target.Embedding, record.Embedding)
		if score > 0 {
			scored = append(scored, similarScore{ArticleID: record.ArticleID, Score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}

	result := make([]SimilarArticleDTO, 0, len(scored))
	for _, item := range scored {
		row, err := s.articles.FindByUserAndID(ctx, req.UserID, item.ArticleID)
		if err != nil {
			continue
		}
		result = append(result, SimilarArticleDTO{
			ArticleID: row.ID,
			Title:     row.Title,
			Summary:   firstNonEmpty(row.Summary, truncateText(row.Content, 220)),
			URL:       row.URL,
			Score:     item.Score,
		})
	}
	return result, nil
}

func (s *Service) GenerateDigest(ctx context.Context, req GenerateDigestRequest) (*DigestDTO, error) {
	rows, _, err := s.articles.ListByUser(ctx, article.ListArticlesParams{
		UserID: req.UserID,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("list digest articles: %w", err)
	}
	inputs := make([]ArticleInput, 0, len(rows))
	for _, row := range rows {
		inputs = append(inputs, articleInput(row))
	}
	assistant, err := s.assistantForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	generated, err := assistant.Digest(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("generate digest: %w", err)
	}
	record, err := s.repo.UpsertDigest(ctx, UpsertDigestParams{
		UserID:        req.UserID,
		DigestDate:    req.Date,
		Model:         generated.Model,
		PromptVersion: generated.PromptVersion,
		Summary:       generated.Summary,
		ArticleIDs:    generated.ArticleIDs,
		Now:           s.now(),
	})
	if err != nil {
		return nil, err
	}
	dto := digestToDTO(*record)
	return &dto, nil
}

func (s *Service) GetDigest(ctx context.Context, req GetDigestRequest) (*DigestDTO, error) {
	record, err := s.repo.FindDigest(ctx, req.UserID, req.Date, DefaultDigestPrompt)
	if err != nil {
		return nil, err
	}
	dto := digestToDTO(*record)
	return &dto, nil
}

func (s *Service) RAGSearch(ctx context.Context, req RAGSearchRequest) (*RAGAnswerDTO, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, ErrEmptyQuery
	}
	limit := normalizeLimit(req.Limit, 5, 12)
	assistant, err := s.assistantForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	if inputs, err := s.embeddingRAGInputs(ctx, assistant, req.UserID, query, limit); err != nil {
		return nil, err
	} else if len(inputs) > 0 {
		result, err := assistant.Answer(ctx, query, inputs)
		if err != nil {
			return nil, fmt.Errorf("generate rag answer: %w", err)
		}
		return &RAGAnswerDTO{
			Model:         result.Model,
			PromptVersion: result.PromptVersion,
			Answer:        result.Answer,
			Citations:     result.Citations,
		}, nil
	}

	rows, _, err := s.articles.ListByUser(ctx, article.ListArticlesParams{
		UserID: req.UserID,
		Query:  query,
		Limit:  limit,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("list rag articles: %w", err)
	}
	inputs := make([]ArticleInput, 0, len(rows))
	for _, row := range rows {
		inputs = append(inputs, articleInput(row))
	}
	result, err := assistant.Answer(ctx, query, inputs)
	if err != nil {
		return nil, fmt.Errorf("generate rag answer: %w", err)
	}
	return &RAGAnswerDTO{
		Model:         result.Model,
		PromptVersion: result.PromptVersion,
		Answer:        result.Answer,
		Citations:     result.Citations,
	}, nil
}

func (s *Service) embeddingRAGInputs(ctx context.Context, assistant Assistant, userID int64, query string, limit int) ([]ArticleInput, error) {
	queryEmbedding, err := assistant.Embed(ctx, query)
	if err != nil {
		return nil, nil
	}
	records, err := s.repo.ListEmbeddingsByUser(ctx, userID, queryEmbedding.Model, queryEmbedding.Version, 500)
	if err != nil {
		return nil, fmt.Errorf("list rag embeddings: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}

	scored := make([]similarScore, 0, len(records))
	for _, record := range records {
		score := cosineSimilarity(queryEmbedding.Vector, record.Embedding)
		if score > 0 {
			scored = append(scored, similarScore{ArticleID: record.ArticleID, Score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}

	inputs := make([]ArticleInput, 0, len(scored))
	for _, item := range scored {
		row, err := s.articles.FindByUserAndID(ctx, userID, item.ArticleID)
		if err != nil {
			continue
		}
		inputs = append(inputs, articleInput(row))
	}
	return inputs, nil
}

func (s *Service) assistantForUser(ctx context.Context, userID int64) (Assistant, error) {
	record, err := s.repo.FindAISettings(ctx, userID)
	if errors.Is(err, ErrAISettingsNotFound) {
		return s.assistant, nil
	}
	if err != nil {
		return nil, err
	}
	provider, err := normalizeAIProvider(record.Provider)
	if err != nil {
		return nil, err
	}
	if provider == "local" {
		return s.assistant, nil
	}
	if len(record.APIKeyCiphertext) == 0 || len(record.APIKeyNonce) == 0 {
		return nil, ErrAISettingsEncryptionKeyRequired
	}
	if s.secretBox == nil {
		return nil, ErrAISettingsEncryptionKeyRequired
	}
	apiKey, err := s.secretBox.DecryptString(record.APIKeyCiphertext, record.APIKeyNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypt ai api key: %w", err)
	}
	baseURL, err := normalizeAIBaseURL(record.BaseURL)
	if err != nil {
		return nil, err
	}
	assistant, err := s.assistantFactory(OpenAIConfig{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		ChatModel:      record.Model,
		EmbeddingModel: firstNonEmpty(record.EmbeddingModel, DefaultOpenAIEmbeddingModel),
	})
	if err != nil {
		return nil, err
	}
	return assistant, nil
}

func defaultAssistantFactory(cfg OpenAIConfig) (Assistant, error) {
	return NewOpenAICompatibleAssistant(cfg)
}

func (s *Service) nextAttempt(attempts int) time.Time {
	delay := time.Duration(attempts+1) * s.retryBackoff
	return s.now().Add(delay)
}

type similarScore struct {
	ArticleID int64
	Score     float64
}

var ErrEmptyQuery = errors.New("empty query")

func articleInput(row article.ArticleWithState) ArticleInput {
	return ArticleInput{
		ID:          row.ID,
		Title:       row.Title,
		Summary:     row.Summary,
		Content:     row.Content,
		URL:         row.URL,
		ContentHash: contentHash(row),
	}
}

func embeddingText(row article.ArticleWithState) string {
	return strings.Join([]string{row.Title, row.Summary, row.Content}, "\n")
}

func contentHash(row article.ArticleWithState) string {
	sum := sha256.Sum256([]byte(embeddingText(row)))
	return fmt.Sprintf("%x", sum)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func normalizeLimit(value, defaultValue, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func summaryToDTO(summary ArticleSummary) SummaryDTO {
	return SummaryDTO{
		ID:            summary.ID,
		ArticleID:     summary.ArticleID,
		Model:         summary.Model,
		PromptVersion: summary.PromptVersion,
		Summary:       summary.Summary,
		Status:        summary.Status,
		Attempts:      summary.Attempts,
		ErrorMessage:  summary.ErrorMessage,
		CreatedAt:     summary.CreatedAt,
		UpdatedAt:     summary.UpdatedAt,
	}
}

func embeddingToDTO(record EmbeddingRecord) EmbeddingDTO {
	return EmbeddingDTO{
		ID:          record.ID,
		ArticleID:   record.ArticleID,
		Model:       record.Model,
		Version:     record.Version,
		Dimensions:  record.Dimensions,
		ContentHash: record.ContentHash,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func aiSettingsToDTO(record UserAISettingsRecord) AISettingsDTO {
	provider, err := normalizeAIProvider(record.Provider)
	if err != nil {
		provider = "local"
	}
	return AISettingsDTO{
		Provider:       provider,
		BaseURL:        record.BaseURL,
		Model:          record.Model,
		EmbeddingModel: record.EmbeddingModel,
		HasAPIKey:      len(record.APIKeyCiphertext) > 0 && len(record.APIKeyNonce) > 0,
		UpdatedAt:      record.UpdatedAt,
	}
}

func defaultAISettingsDTO() AISettingsDTO {
	return AISettingsDTO{
		Provider:       "local",
		BaseURL:        DefaultOpenAIBaseURL,
		EmbeddingModel: DefaultOpenAIEmbeddingModel,
		HasAPIKey:      false,
	}
}

func normalizeAIProvider(provider string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "local", "extractive":
		return "local", nil
	case "openai", "openai-compatible":
		return "openai-compatible", nil
	default:
		return "", ErrInvalidAIProvider
	}
}

func normalizeAIBaseURL(raw string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}
	if err := netguard.ValidateHTTPURL(baseURL); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidAIBaseURL, err)
	}
	return baseURL, nil
}

func digestToDTO(record DigestRecord) DigestDTO {
	return DigestDTO{
		ID:            record.ID,
		DigestDate:    record.DigestDate.Format(time.DateOnly),
		Model:         record.Model,
		PromptVersion: record.PromptVersion,
		Summary:       record.Summary,
		ArticleIDs:    record.ArticleIDs,
		Status:        record.Status,
		ErrorMessage:  record.ErrorMessage,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}
