package ai

import (
	"context"
	"crypto/sha256"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type Assistant interface {
	Summarize(ctx context.Context, article ArticleInput) (SummaryResult, error)
	Embed(ctx context.Context, text string) (EmbeddingResult, error)
	Digest(ctx context.Context, articles []ArticleInput) (DigestResult, error)
	Answer(ctx context.Context, query string, articles []ArticleInput) (RAGResult, error)
}

type ArticleInput struct {
	ID          int64
	Title       string
	Summary     string
	Content     string
	URL         *string
	ContentHash string
}

type SummaryResult struct {
	Model         string
	PromptVersion string
	Summary       string
}

type EmbeddingResult struct {
	Model      string
	Version    string
	Dimensions int
	Vector     []float64
}

type DigestResult struct {
	Model         string
	PromptVersion string
	Summary       string
	ArticleIDs    []int64
}

type RAGResult struct {
	Model         string
	PromptVersion string
	Answer        string
	Citations     []CitationDTO
}

type ExtractiveAssistant struct {
	dimensions int
}

func NewExtractiveAssistant() *ExtractiveAssistant {
	return &ExtractiveAssistant{dimensions: DefaultEmbeddingDimension}
}

func (a *ExtractiveAssistant) Summarize(_ context.Context, article ArticleInput) (SummaryResult, error) {
	text := strings.TrimSpace(firstNonEmpty(article.Content, article.Summary, article.Title))
	sentences := splitSentences(text)
	if len(sentences) > 3 {
		sentences = sentences[:3]
	}
	summary := strings.TrimSpace(strings.Join(sentences, " "))
	if summary == "" {
		summary = strings.TrimSpace(article.Title)
	}
	return SummaryResult{
		Model:         DefaultSummaryModel,
		PromptVersion: DefaultSummaryPrompt,
		Summary:       truncateText(summary, 900),
	}, nil
}

func (a *ExtractiveAssistant) Embed(_ context.Context, text string) (EmbeddingResult, error) {
	dimensions := a.dimensions
	if dimensions <= 0 {
		dimensions = DefaultEmbeddingDimension
	}
	vector := make([]float64, dimensions)
	for _, token := range tokenize(text) {
		hash := sha256.Sum256([]byte(token))
		idx := int(hash[0]) % dimensions
		weight := 1.0 + float64(len(token)%7)/10.0
		vector[idx] += weight
	}
	normalize(vector)
	return EmbeddingResult{
		Model:      DefaultEmbeddingModel,
		Version:    DefaultEmbeddingVersion,
		Dimensions: dimensions,
		Vector:     vector,
	}, nil
}

func (a *ExtractiveAssistant) Digest(_ context.Context, articles []ArticleInput) (DigestResult, error) {
	if len(articles) == 0 {
		return DigestResult{
			Model:         DefaultSummaryModel,
			PromptVersion: DefaultDigestPrompt,
			Summary:       "今天没有可汇总的文章。",
			ArticleIDs:    []int64{},
		}, nil
	}

	limit := len(articles)
	if limit > 5 {
		limit = 5
	}

	lines := make([]string, 0, limit)
	ids := make([]int64, 0, limit)
	for _, article := range articles[:limit] {
		lines = append(lines, "- "+truncateText(strings.TrimSpace(article.Title), 120))
		ids = append(ids, article.ID)
	}
	return DigestResult{
		Model:         DefaultSummaryModel,
		PromptVersion: DefaultDigestPrompt,
		Summary:       strings.Join(lines, "\n"),
		ArticleIDs:    ids,
	}, nil
}

func (a *ExtractiveAssistant) Answer(_ context.Context, query string, articles []ArticleInput) (RAGResult, error) {
	if len(articles) == 0 {
		return RAGResult{
			Model:         DefaultSummaryModel,
			PromptVersion: DefaultRAGPrompt,
			Answer:        "没有找到可引用的相关文章。",
			Citations:     []CitationDTO{},
		}, nil
	}

	queryTokens := tokenSet(query)
	sort.SliceStable(articles, func(i, j int) bool {
		return overlapScore(queryTokens, articles[i]) > overlapScore(queryTokens, articles[j])
	})

	limit := len(articles)
	if limit > 3 {
		limit = 3
	}

	parts := make([]string, 0, limit)
	citations := make([]CitationDTO, 0, limit)
	for _, article := range articles[:limit] {
		snippet := truncateText(firstNonEmpty(article.Summary, article.Content, article.Title), 220)
		parts = append(parts, snippet)
		citations = append(citations, CitationDTO{
			ArticleID: article.ID,
			Title:     article.Title,
			URL:       article.URL,
			Snippet:   snippet,
		})
	}
	return RAGResult{
		Model:         DefaultSummaryModel,
		PromptVersion: DefaultRAGPrompt,
		Answer:        strings.Join(parts, "\n\n"),
		Citations:     citations,
	}, nil
}

func splitSentences(text string) []string {
	fields := regexp.MustCompile(`[。！？.!?]\s*`).Split(text, -1)
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			result = append(result, trimmed+"。")
		}
	}
	return result
}

func tokenize(text string) []string {
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len([]rune(part)) >= 2 {
			result = append(result, part)
		}
	}
	return result
}

func tokenSet(text string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, token := range tokenize(text) {
		result[token] = struct{}{}
	}
	return result
}

func overlapScore(tokens map[string]struct{}, article ArticleInput) int {
	score := 0
	text := strings.ToLower(article.Title + " " + article.Summary + " " + article.Content)
	for token := range tokens {
		if strings.Contains(text, token) {
			score++
		}
	}
	return score
}

func normalize(vector []float64) {
	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	if sum == 0 {
		return
	}
	length := math.Sqrt(sum)
	for i := range vector {
		vector[i] = vector[i] / length
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateText(text string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}
