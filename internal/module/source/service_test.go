package source_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/source"
	sourcemocks "github.com/tepzxl/contentflow/internal/module/source/mocks"
	"go.uber.org/mock/gomock"
	"gorm.io/datatypes"
)

func TestSourceService_CreateSource(t *testing.T) {
	now := fixedTime()

	tests := []struct {
		name    string
		req     source.CreateSourceRequest
		mock    func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository)
		assert  func(t *testing.T, resp *source.CreateSourceResponse)
		wantErr error
	}{
		{
			name: "create rss source success",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   " Go Blog ",
				Type:   "RSS",
				URL:    strPtr("https://go.dev/blog/feed.atom"),
				Config: json.RawMessage(`{}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&source.Source{})).
					DoAndReturn(func(_ context.Context, s *source.Source) error {
						if s.UserID != 100 {
							t.Fatalf("UserID = %d, want 100", s.UserID)
						}

						if s.Name != "Go Blog" {
							t.Fatalf("Name = %s, want Go Blog", s.Name)
						}

						if s.Type != source.TypeRSS {
							t.Fatalf("Type = %s, want rss", s.Type)
						}

						if s.URL == nil || *s.URL != "https://go.dev/blog/feed.atom" {
							t.Fatalf("unexpected URL: %v", s.URL)
						}

						if string(s.ConfigJSON) != `{}` {
							t.Fatalf("ConfigJSON = %s, want {}", string(s.ConfigJSON))
						}

						if !s.IsActive {
							t.Fatal("IsActive should be true")
						}

						if !s.CreatedAt.Equal(now) {
							t.Fatalf("CreatedAt = %v, want %v", s.CreatedAt, now)
						}

						if !s.UpdatedAt.Equal(now) {
							t.Fatalf("UpdatedAt = %v, want %v", s.UpdatedAt, now)
						}

						s.ID = 1
						return nil
					})
			},
			wantErr: nil,
		},
		{
			name: "create email source success without url",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Newsletter",
				Type:   "email",
				URL:    nil,
				Config: json.RawMessage(`{"provider":"shared_mailbox"}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&source.Source{})).
					DoAndReturn(func(_ context.Context, s *source.Source) error {
						if s.Type != source.TypeEmail {
							t.Fatalf("Type = %s, want email", s.Type)
						}

						if s.URL != nil {
							t.Fatalf("URL = %v, want nil", s.URL)
						}

						if string(s.ConfigJSON) != `{"provider":"shared_mailbox"}` {
							t.Fatalf("ConfigJSON = %s", string(s.ConfigJSON))
						}

						s.ID = 2
						return nil
					})
			},
			wantErr: nil,
		},
		{
			name: "create email source redacts secret config in response",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Inbox",
				Type:   "email",
				Config: json.RawMessage(`{"provider":"imap","username":"reader","password":"mail-secret","nested":{"api_key":"api-secret"}}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&source.Source{})).
					DoAndReturn(func(_ context.Context, s *source.Source) error {
						if !bytes.Contains(s.ConfigJSON, []byte("mail-secret")) {
							t.Fatalf("stored ConfigJSON = %s, want raw secret preserved for collector", string(s.ConfigJSON))
						}
						s.ID = 3
						return nil
					})
			},
			assert: func(t *testing.T, resp *source.CreateSourceResponse) {
				t.Helper()
				assertRedactedConfig(t, resp.Source.Config)
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "   ",
				Type:   source.TypeRSS,
				URL:    strPtr("https://go.dev/blog/feed.atom"),
				Config: json.RawMessage(`{}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantErr: source.ErrInvalidSourceName,
		},
		{
			name: "invalid source type",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Bad Source",
				Type:   "unknown",
				URL:    nil,
				Config: json.RawMessage(`{}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantErr: source.ErrInvalidSourceType,
		},
		{
			name: "rss source missing url",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Go Blog",
				Type:   source.TypeRSS,
				URL:    nil,
				Config: json.RawMessage(`{}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantErr: source.ErrInvalidSourceURL,
		},
		{
			name: "invalid url",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Go Blog",
				Type:   source.TypeRSS,
				URL:    strPtr("not-a-url"),
				Config: json.RawMessage(`{}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantErr: source.ErrInvalidSourceURL,
		},
		{
			name: "rss source rejects private address url",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Metadata",
				Type:   source.TypeRSS,
				URL:    strPtr("http://169.254.169.254/latest/meta-data"),
				Config: json.RawMessage(`{}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantErr: source.ErrInvalidSourceURL,
		},
		{
			name: "invalid config json",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Go Blog",
				Type:   source.TypeRSS,
				URL:    strPtr("https://go.dev/blog/feed.atom"),
				Config: json.RawMessage(`{`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantErr: source.ErrInvalidSourceConfig,
		},
		{
			name: "source url duplicated",
			req: source.CreateSourceRequest{
				UserID: 100,
				Name:   "Go Blog",
				Type:   source.TypeRSS,
				URL:    strPtr("https://go.dev/blog/feed.atom"),
				Config: json.RawMessage(`{}`),
			},
			mock: func(t *testing.T, ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&source.Source{})).
					Return(source.ErrSourceURLDuplicated)
			},
			wantErr: source.ErrSourceAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := sourcemocks.NewMockRepository(ctrl)

			tt.mock(t, ctx, repo)

			svc := source.NewService(
				repo,
				source.WithNow(func() time.Time {
					return now
				}),
			)

			resp, err := svc.CreateSource(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("CreateSource() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateSource() unexpected error = %v", err)
			}

			if resp == nil {
				t.Fatal("CreateSource() response is nil")
			}

			if resp.Source.ID == 0 {
				t.Fatal("CreateSource() source id should not be zero")
			}
			if tt.assert != nil {
				tt.assert(t, resp)
			}
		})
	}
}

func TestSourceService_ListSources(t *testing.T) {
	model := sampleSourceModel()

	tests := []struct {
		name    string
		req     source.ListSourcesRequest
		mock    func(ctx context.Context, repo *sourcemocks.MockRepository)
		assert  func(t *testing.T, resp *source.ListSourcesResponse)
		wantErr error
	}{
		{
			name: "default pagination",
			req: source.ListSourcesRequest{
				UserID: 100,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					ListByUserID(ctx, source.ListParams{
						UserID: 100,
						Type:   "",
						Limit:  20,
						Offset: 0,
					}).
					Return([]source.Source{*model}, int64(1), nil)
			},
			wantErr: nil,
		},
		{
			name: "with type limit offset",
			req: source.ListSourcesRequest{
				UserID: 100,
				Type:   "RSS",
				Limit:  10,
				Offset: 20,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					ListByUserID(ctx, source.ListParams{
						UserID: 100,
						Type:   source.TypeRSS,
						Limit:  10,
						Offset: 20,
					}).
					Return([]source.Source{*model}, int64(1), nil)
			},
			wantErr: nil,
		},
		{
			name: "list redacts secret config",
			req: source.ListSourcesRequest{
				UserID: 100,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				model := *sampleSourceModel()
				model.ConfigJSON = datatypes.JSON([]byte(`{"provider":"imap","password":"mail-secret","token":"feed-token"}`))
				repo.EXPECT().
					ListByUserID(ctx, source.ListParams{
						UserID: 100,
						Type:   "",
						Limit:  20,
						Offset: 0,
					}).
					Return([]source.Source{model}, int64(1), nil)
			},
			assert: func(t *testing.T, resp *source.ListSourcesResponse) {
				t.Helper()
				if len(resp.Sources) != 1 {
					t.Fatalf("len(Sources) = %d, want 1", len(resp.Sources))
				}
				assertRedactedConfig(t, resp.Sources[0].Config)
			},
			wantErr: nil,
		},
		{
			name: "limit too large",
			req: source.ListSourcesRequest{
				UserID: 100,
				Limit:  1000,
				Offset: 0,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					ListByUserID(ctx, source.ListParams{
						UserID: 100,
						Type:   "",
						Limit:  100,
						Offset: 0,
					}).
					Return([]source.Source{}, int64(0), nil)
			},
			wantErr: nil,
		},
		{
			name: "negative offset",
			req: source.ListSourcesRequest{
				UserID: 100,
				Limit:  10,
				Offset: -10,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					ListByUserID(ctx, source.ListParams{
						UserID: 100,
						Type:   "",
						Limit:  10,
						Offset: 0,
					}).
					Return([]source.Source{}, int64(0), nil)
			},
			wantErr: nil,
		},
		{
			name: "invalid source type",
			req: source.ListSourcesRequest{
				UserID: 100,
				Type:   "unknown",
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantErr: source.ErrInvalidSourceType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := sourcemocks.NewMockRepository(ctrl)

			tt.mock(ctx, repo)

			svc := source.NewService(repo)

			resp, err := svc.ListSources(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ListSources() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ListSources() unexpected error = %v", err)
			}

			if resp == nil {
				t.Fatal("ListSources() response is nil")
			}
			if tt.assert != nil {
				tt.assert(t, resp)
			}
		})
	}
}

func TestSourceService_ListSourcesCache(t *testing.T) {
	model := sampleSourceDTO()
	rawSecretModel := model
	rawSecretModel.Config = json.RawMessage(`{"password":"mail-secret"}`)

	tests := []struct {
		name      string
		cache     *fakeListCache
		mock      func(ctx context.Context, repo *sourcemocks.MockRepository)
		assert    func(t *testing.T, resp *source.ListSourcesResponse)
		wantSets  int
		wantTotal int64
	}{
		{
			name: "cache hit skips repository",
			cache: &fakeListCache{
				resp: &source.ListSourcesResponse{
					Sources: []source.SourceDTO{model},
					Total:   1,
					Limit:   20,
					Offset:  0,
				},
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			wantSets:  0,
			wantTotal: 1,
		},
		{
			name: "cache hit redacts legacy secret config",
			cache: &fakeListCache{
				resp: &source.ListSourcesResponse{
					Sources: []source.SourceDTO{rawSecretModel},
					Total:   1,
					Limit:   20,
					Offset:  0,
				},
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
			},
			assert: func(t *testing.T, resp *source.ListSourcesResponse) {
				t.Helper()
				assertRedactedConfig(t, resp.Sources[0].Config)
			},
			wantSets:  0,
			wantTotal: 1,
		},
		{
			name:  "cache miss writes cache",
			cache: &fakeListCache{},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					ListByUserID(ctx, source.ListParams{
						UserID: 100,
						Type:   "",
						Limit:  20,
						Offset: 0,
					}).
					Return([]source.Source{*sampleSourceModel()}, int64(1), nil)
			},
			wantSets:  1,
			wantTotal: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := sourcemocks.NewMockRepository(ctrl)
			tt.mock(ctx, repo)

			svc := source.NewService(repo, source.WithListCache(tt.cache, time.Minute))

			resp, err := svc.ListSources(ctx, source.ListSourcesRequest{UserID: 100})
			if err != nil {
				t.Fatalf("ListSources() error = %v", err)
			}
			if resp.Total != tt.wantTotal {
				t.Fatalf("Total = %d, want %d", resp.Total, tt.wantTotal)
			}
			if tt.cache.sets != tt.wantSets {
				t.Fatalf("cache sets = %d, want %d", tt.cache.sets, tt.wantSets)
			}
			if tt.assert != nil {
				tt.assert(t, resp)
			}
		})
	}
}

func TestSourceService_GetSource(t *testing.T) {
	model := sampleSourceModel()

	tests := []struct {
		name    string
		req     source.GetSourceRequest
		mock    func(ctx context.Context, repo *sourcemocks.MockRepository)
		wantErr error
	}{
		{
			name: "success",
			req: source.GetSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(1)).
					Return(model, nil)
			},
			wantErr: nil,
		},
		{
			name: "not found becomes not accessible",
			req: source.GetSourceRequest{
				UserID:   100,
				SourceID: 999,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(999)).
					Return(nil, source.ErrSourceNotFound)
			},
			wantErr: source.ErrSourceNotAccessible,
		},
		{
			name: "repository error",
			req: source.GetSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(1)).
					Return(nil, errors.New("db error"))
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := sourcemocks.NewMockRepository(ctrl)

			tt.mock(ctx, repo)

			svc := source.NewService(repo)

			resp, err := svc.GetSource(ctx, tt.req)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("GetSource() expected error %v, got nil", tt.wantErr)
				}

				if errors.Is(tt.wantErr, source.ErrSourceNotAccessible) && !errors.Is(err, source.ErrSourceNotAccessible) {
					t.Fatalf("GetSource() error = %v, want ErrSourceNotAccessible", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("GetSource() unexpected error = %v", err)
			}

			if resp == nil {
				t.Fatal("GetSource() response is nil")
			}
		})
	}
}

func TestSourceService_DeleteSource(t *testing.T) {
	now := fixedTime()

	tests := []struct {
		name    string
		req     source.DeleteSourceRequest
		mock    func(ctx context.Context, repo *sourcemocks.MockRepository)
		wantErr error
	}{
		{
			name: "success",
			req: source.DeleteSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					SoftDelete(ctx, int64(100), int64(1), now).
					Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "not found becomes not accessible",
			req: source.DeleteSourceRequest{
				UserID:   100,
				SourceID: 999,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					SoftDelete(ctx, int64(100), int64(999), now).
					Return(source.ErrSourceNotFound)
			},
			wantErr: source.ErrSourceNotAccessible,
		},
		{
			name: "repository error",
			req: source.DeleteSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			mock: func(ctx context.Context, repo *sourcemocks.MockRepository) {
				repo.EXPECT().
					SoftDelete(ctx, int64(100), int64(1), now).
					Return(errors.New("db error"))
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := sourcemocks.NewMockRepository(ctrl)

			tt.mock(ctx, repo)

			svc := source.NewService(
				repo,
				source.WithNow(func() time.Time {
					return now
				}),
			)

			err := svc.DeleteSource(ctx, tt.req)

			if tt.wantErr != nil {
				if errors.Is(tt.wantErr, source.ErrSourceNotAccessible) {
					if !errors.Is(err, source.ErrSourceNotAccessible) {
						t.Fatalf("DeleteSource() error = %v, want ErrSourceNotAccessible", err)
					}
					return
				}

				if err == nil {
					t.Fatalf("DeleteSource() expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("DeleteSource() unexpected error = %v", err)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(v bool) *bool {
	return &v
}

func fixedTime() time.Time {
	return time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
}

func assertRedactedConfig(t *testing.T, config json.RawMessage) {
	t.Helper()
	if bytes.Contains(config, []byte("mail-secret")) || bytes.Contains(config, []byte("api-secret")) || bytes.Contains(config, []byte("feed-token")) {
		t.Fatalf("config leaks secret: %s", string(config))
	}
	if !bytes.Contains(config, []byte("[REDACTED]")) {
		t.Fatalf("config = %s, want redaction marker", string(config))
	}
}

func sampleSourceModel() *source.Source {
	now := fixedTime()
	url := "https://go.dev/blog/feed.atom"

	return &source.Source{
		ID:               1,
		UserID:           100,
		Name:             "Go Blog",
		Type:             source.TypeRSS,
		URL:              &url,
		ConfigJSON:       datatypes.JSON([]byte(`{}`)),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

type fakeListCache struct {
	resp *source.ListSourcesResponse
	sets int
}

func (f *fakeListCache) GetList(ctx context.Context, req source.ListSourcesRequest) (*source.ListSourcesResponse, bool, error) {
	if f.resp == nil {
		return nil, false, nil
	}
	return f.resp, true, nil
}

func (f *fakeListCache) SetList(ctx context.Context, req source.ListSourcesRequest, resp *source.ListSourcesResponse, ttl time.Duration) error {
	f.sets++
	return nil
}

func (f *fakeListCache) DeleteUser(ctx context.Context, userID int64) error {
	return nil
}
