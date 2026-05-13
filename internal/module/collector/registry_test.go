package collector_test

import (
	"strings"
	"testing"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
)

func TestNewRegistry(t *testing.T) {
	tests := []struct {
		name        string
		collectors  []collector.Collector
		wantErr     bool
		errContains string
	}{
		{
			name: "success with rss and email collectors",
			collectors: []collector.Collector{
				fakeCollector{sourceType: source.TypeRSS},
				fakeCollector{sourceType: source.TypeEmail},
			},
			wantErr: false,
		},
		{
			name: "skip nil collector",
			collectors: []collector.Collector{
				nil,
				fakeCollector{sourceType: source.TypeRSS},
			},
			wantErr: false,
		},
		{
			name: "empty collector type",
			collectors: []collector.Collector{
				fakeCollector{sourceType: ""},
			},
			wantErr:     true,
			errContains: "collector type is empty",
		},
		{
			name: "duplicated collector type",
			collectors: []collector.Collector{
				fakeCollector{sourceType: source.TypeRSS},
				fakeCollector{sourceType: source.TypeRSS},
			},
			wantErr:     true,
			errContains: "collector duplicated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := collector.NewRegistry(tt.collectors...)

			if tt.wantErr {
				if err == nil {
					t.Fatal("NewRegistry() expected error, got nil")
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("NewRegistry() error = %v, want contains %q", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Fatalf("NewRegistry() unexpected error = %v", err)
			}

			if registry == nil {
				t.Fatal("NewRegistry() registry is nil")
			}
		})
	}
}

func TestRegistry_Get(t *testing.T) {
	tests := []struct {
		name       string
		collectors []collector.Collector
		sourceType string
		wantFound  bool
		wantType   string
	}{
		{
			name: "get rss collector",
			collectors: []collector.Collector{
				fakeCollector{sourceType: source.TypeRSS},
				fakeCollector{sourceType: source.TypeEmail},
			},
			sourceType: source.TypeRSS,
			wantFound:  true,
			wantType:   source.TypeRSS,
		},
		{
			name: "get email collector",
			collectors: []collector.Collector{
				fakeCollector{sourceType: source.TypeRSS},
				fakeCollector{sourceType: source.TypeEmail},
			},
			sourceType: source.TypeEmail,
			wantFound:  true,
			wantType:   source.TypeEmail,
		},
		{
			name: "collector not found",
			collectors: []collector.Collector{
				fakeCollector{sourceType: source.TypeRSS},
			},
			sourceType: "github",
			wantFound:  false,
		},
		{
			name:       "empty registry",
			collectors: []collector.Collector{},
			sourceType: source.TypeRSS,
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := collector.NewRegistry(tt.collectors...)
			if err != nil {
				t.Fatalf("NewRegistry() error = %v", err)
			}

			got, ok := registry.Get(tt.sourceType)

			if ok != tt.wantFound {
				t.Fatalf("Get() found = %v, want %v", ok, tt.wantFound)
			}

			if !tt.wantFound {
				if got != nil {
					t.Fatalf("Get() collector = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("Get() collector is nil")
			}

			if got.Type() != tt.wantType {
				t.Fatalf("Get() collector type = %s, want %s", got.Type(), tt.wantType)
			}
		})
	}
}
