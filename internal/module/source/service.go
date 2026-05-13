package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/datatypes"
)

var (
	ErrInvalidSourceName   = errors.New("invalid source name")
	ErrInvalidSourceType   = errors.New("invalid source type")
	ErrInvalidSourceURL    = errors.New("invalid source url")
	ErrInvalidSourceConfig = errors.New("invalid source config")
	ErrSourceAlreadyExists = errors.New("source already exists")
	ErrSourceNotAccessible = errors.New("source not accessible")
)

type Service interface {
	CreateSource(ctx context.Context, req CreateSourceRequest) (*CreateSourceResponse, error)
	ListSources(ctx context.Context, req ListSourcesRequest) (*ListSourcesResponse, error)
	GetSource(ctx context.Context, req GetSourceRequest) (*GetSourceResponse, error)
	UpdateSource(ctx context.Context, req UpdateSourceRequest) (*UpdateSourceResponse, error)
	DeleteSource(ctx context.Context, req DeleteSourceRequest) error
}

type SourceService struct {
	repo      Repository
	now       func() time.Time
	listCache ListCache
	cacheTTL  time.Duration
}

type Option func(*SourceService)

func WithNow(now func() time.Time) Option {
	return func(s *SourceService) {
		s.now = now
	}
}

type ListCache interface {
	GetList(ctx context.Context, req ListSourcesRequest) (*ListSourcesResponse, bool, error)
	SetList(ctx context.Context, req ListSourcesRequest, resp *ListSourcesResponse, ttl time.Duration) error
	DeleteUser(ctx context.Context, userID int64) error
}

func WithListCache(cache ListCache, ttl time.Duration) Option {
	return func(s *SourceService) {
		if cache != nil && ttl > 0 {
			s.listCache = cache
			s.cacheTTL = ttl
		}
	}
}

func NewService(repo Repository, opts ...Option) Service {
	s := &SourceService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *SourceService) CreateSource(ctx context.Context, req CreateSourceRequest) (*CreateSourceResponse, error) {
	name, err := normalizeName(req.Name)
	if err != nil {
		return nil, err
	}

	sourceType := strings.TrimSpace(strings.ToLower(req.Type))
	if !IsValidType(sourceType) {
		return nil, ErrInvalidSourceType
	}

	normalizedURL, err := normalizeURL(req.URL)
	if err != nil {
		return nil, err
	}

	if sourceType == TypeRSS && normalizedURL == nil {
		return nil, ErrInvalidSourceURL
	}

	configJSON, err := normalizeConfig(req.Config)
	if err != nil {
		return nil, err
	}

	now := s.now()

	src := &Source{
		UserID:           req.UserID,
		Name:             name,
		Type:             sourceType,
		URL:              normalizedURL,
		ConfigJSON:       datatypes.JSON(configJSON),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.repo.Create(ctx, src); err != nil {
		if errors.Is(err, ErrSourceURLDuplicated) {
			return nil, ErrSourceAlreadyExists
		}

		return nil, fmt.Errorf("create source: %w", err)
	}
	s.deleteUserListCache(ctx, req.UserID)

	return &CreateSourceResponse{
		Source: toDTO(src),
	}, nil
}

func (s *SourceService) ListSources(ctx context.Context, req ListSourcesRequest) (*ListSourcesResponse, error) {
	sourceType := strings.TrimSpace(strings.ToLower(req.Type))
	if sourceType != "" && !IsValidType(sourceType) {
		return nil, ErrInvalidSourceType
	}

	limit := normalizeLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	normalizedReq := ListSourcesRequest{
		UserID: req.UserID,
		Type:   sourceType,
		Limit:  limit,
		Offset: offset,
	}

	if s.listCache != nil {
		if cached, ok, err := s.listCache.GetList(ctx, normalizedReq); err == nil && ok {
			return cached, nil
		}
	}

	sources, total, err := s.repo.ListByUserID(ctx, ListParams{
		UserID: normalizedReq.UserID,
		Type:   sourceType,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	dtos := make([]SourceDTO, 0, len(sources))
	for i := range sources {
		dtos = append(dtos, toDTO(&sources[i]))
	}

	resp := &ListSourcesResponse{
		Sources: dtos,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}

	if s.listCache != nil {
		_ = s.listCache.SetList(ctx, normalizedReq, resp, s.cacheTTL)
	}

	return resp, nil
}

func (s *SourceService) GetSource(ctx context.Context, req GetSourceRequest) (*GetSourceResponse, error) {
	src, err := s.repo.FindByUserIDAndID(ctx, req.UserID, req.SourceID)
	if errors.Is(err, ErrSourceNotFound) {
		return nil, ErrSourceNotAccessible
	}

	if err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}

	return &GetSourceResponse{
		Source: toDTO(src),
	}, nil
}

func (s *SourceService) UpdateSource(ctx context.Context, req UpdateSourceRequest) (*UpdateSourceResponse, error) {
	src, err := s.repo.FindByUserIDAndID(ctx, req.UserID, req.SourceID)
	if errors.Is(err, ErrSourceNotFound) {
		return nil, ErrSourceNotAccessible
	}

	if err != nil {
		return nil, fmt.Errorf("find source before update: %w", err)
	}

	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return nil, err
		}
		src.Name = name
	}

	if req.URL != nil {
		normalizedURL, err := normalizeURL(req.URL)
		if err != nil {
			return nil, err
		}

		if src.Type == TypeRSS && normalizedURL == nil {
			return nil, ErrInvalidSourceURL
		}

		src.URL = normalizedURL
	}

	if req.Config != nil {
		configJSON, err := normalizeConfig(req.Config)
		if err != nil {
			return nil, err
		}
		src.ConfigJSON = datatypes.JSON(configJSON)
	}

	if req.IsActive != nil {
		src.IsActive = *req.IsActive
	}

	src.UpdatedAt = s.now()

	if err := s.repo.Update(ctx, src); err != nil {
		if errors.Is(err, ErrSourceNotFound) {
			return nil, ErrSourceNotAccessible
		}

		if errors.Is(err, ErrSourceURLDuplicated) {
			return nil, ErrSourceAlreadyExists
		}

		return nil, fmt.Errorf("update source: %w", err)
	}
	s.deleteUserListCache(ctx, req.UserID)

	return &UpdateSourceResponse{
		Source: toDTO(src),
	}, nil
}

func (s *SourceService) DeleteSource(ctx context.Context, req DeleteSourceRequest) error {
	err := s.repo.SoftDelete(ctx, req.UserID, req.SourceID, s.now())
	if errors.Is(err, ErrSourceNotFound) {
		return ErrSourceNotAccessible
	}

	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	s.deleteUserListCache(ctx, req.UserID)

	return nil
}

func (s *SourceService) deleteUserListCache(ctx context.Context, userID int64) {
	if s.listCache != nil {
		_ = s.listCache.DeleteUser(ctx, userID)
	}
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidSourceName
	}

	if len(name) > 200 {
		return "", ErrInvalidSourceName
	}

	return name, nil
}

func normalizeURL(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return nil, ErrInvalidSourceURL
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidSourceURL
	}

	return &value, nil
}

func normalizeConfig(config json.RawMessage) ([]byte, error) {
	if len(config) == 0 {
		return []byte(`{}`), nil
	}

	if !json.Valid(config) {
		return nil, ErrInvalidSourceConfig
	}

	return config, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}

	if limit > 100 {
		return 100
	}

	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}

	return offset
}

func toDTO(src *Source) SourceDTO {
	return SourceDTO{
		ID:               src.ID,
		Name:             src.Name,
		Type:             src.Type,
		URL:              src.URL,
		Config:           json.RawMessage(src.ConfigJSON),
		IsActive:         src.IsActive,
		LastFetchedAt:    src.LastFetchedAt,
		LastFetchStatus:  src.LastFetchStatus,
		LastFetchMessage: src.LastFetchMessage,
		CreatedAt:        src.CreatedAt,
		UpdatedAt:        src.UpdatedAt,
	}
}
