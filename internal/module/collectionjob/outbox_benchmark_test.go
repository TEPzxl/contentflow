package collectionjob

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkOutboxDispatcher_DispatchReady(b *testing.B) {
	now := time.Date(2026, 5, 14, 11, 0, 0, 0, time.UTC)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		repo := newMemoryOutboxRepository()
		for j := 0; j < 100; j++ {
			if _, err := repo.Create(ctx, CreateOutboxEventParams{
				Topic:     TopicCollectionRequested,
				Key:       fmt.Sprintf("collection:source:%d", j),
				Payload:   CollectionRequested{TaskID: fmt.Sprintf("task-%d", j), SourceID: int64(j)},
				CreatedAt: now,
			}); err != nil {
				b.Fatalf("Create() error = %v", err)
			}
		}
		dispatcher := NewOutboxDispatcher(
			repo,
			&fakeEventWriter{},
			WithOutboxNow(func() time.Time { return now }),
			WithOutboxBatchSize(100),
		)
		if err := dispatcher.DispatchReady(ctx); err != nil {
			b.Fatalf("DispatchReady() error = %v", err)
		}
	}
}
