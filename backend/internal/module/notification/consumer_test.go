package notification

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConsumerRunOnceProcessesEventAndRunHonorsCancellation(t *testing.T) {
	store := &fakeEventStore{lease: testLease(t), found: true}
	consumer := NewConsumer(NewExpander(store, fakeRecipients{}))
	if found, err := consumer.RunOnce(context.Background(), time.Now()); err != nil || !found || len(store.completed) != 1 {
		t.Fatalf("found=%v err=%v completed=%v", found, err, store.completed)
	}
	if found, err := consumer.RunOnce(context.Background(), time.Now()); err != nil || found {
		t.Fatalf("empty poll found=%v err=%v", found, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := consumer.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
