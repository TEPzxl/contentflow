package observability_test

import (
	"context"
	"testing"

	"github.com/tepzxl/contentflow/internal/observability"
	"go.opentelemetry.io/otel"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

func TestInitTracing_DisabledReturnsNoopShutdown(t *testing.T) {
	otel.SetTracerProvider(nooptrace.NewTracerProvider())

	shutdown, err := observability.InitTracing(context.Background(), observability.TracingConfig{
		Enabled:     false,
		ServiceName: "contentflow",
	})
	if err != nil {
		t.Fatalf("InitTracing() error = %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown is nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}
