package app

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
)

func TestStartBackgroundTaskReportsError(t *testing.T) {
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	sentinelErr := errors.New("background task failed")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	startBackgroundTask(&wg, errCh, log, "test task", func() error {
		return sentinelErr
	})
	wg.Wait()

	select {
	case got := <-errCh:
		if !errors.Is(got, sentinelErr) {
			t.Fatalf("reported error = %v, want %v", got, sentinelErr)
		}
	default:
		t.Fatal("expected background task error to be reported")
	}
}

func TestStartBackgroundTaskIgnoresNilError(t *testing.T) {
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	startBackgroundTask(&wg, errCh, log, "test task", func() error {
		return nil
	})
	wg.Wait()

	select {
	case got := <-errCh:
		t.Fatalf("unexpected reported error: %v", got)
	default:
	}
}

func TestRuntimePlanRunsOutboxDispatcher(t *testing.T) {
	tests := []struct {
		name string
		plan runtimePlan
		want bool
	}{
		{name: "api mode does not run outbox dispatcher", plan: runtimePlan{HTTP: true}, want: false},
		{name: "worker mode does not run outbox dispatcher", plan: runtimePlan{Worker: true}, want: false},
		{name: "scheduler mode runs outbox dispatcher", plan: runtimePlan{Scheduler: true}, want: true},
		{name: "all mode runs outbox dispatcher", plan: runtimePlan{HTTP: true, Scheduler: true, Worker: true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.runsOutboxDispatcher(); got != tt.want {
				t.Fatalf("runsOutboxDispatcher() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuntimePlanForMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		wantHTTP  bool
		wantSched bool
		wantWork  bool
		wantErr   bool
	}{
		{name: "empty defaults to all", mode: "", wantHTTP: true, wantSched: true, wantWork: true},
		{name: "all starts every runtime", mode: "all", wantHTTP: true, wantSched: true, wantWork: true},
		{name: "api only starts http", mode: "api", wantHTTP: true},
		{name: "worker only starts worker", mode: "worker", wantWork: true},
		{name: "scheduler only starts scheduler", mode: "scheduler", wantSched: true},
		{name: "invalid mode", mode: "bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runtimePlanForMode(tt.mode)
			if tt.wantErr {
				if err == nil {
					t.Fatal("runtimePlanForMode() error is nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("runtimePlanForMode() error = %v", err)
			}
			if got.HTTP != tt.wantHTTP || got.Scheduler != tt.wantSched || got.Worker != tt.wantWork {
				t.Fatalf("plan = %+v", got)
			}
		})
	}
}
