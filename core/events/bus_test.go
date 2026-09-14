package events

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPublishCallsAllHandlers(t *testing.T) {
	b := New()
	calls := 0
	_ = b.Subscribe("host.created", func(context.Context, Event) error { calls++; return nil })
	_ = b.Subscribe("host.created", func(context.Context, Event) error { calls++; return errors.New("failure") })
	err := b.Publish(context.Background(), Event{Name: "host.created", Version: 1, OccurredAt: time.Now()})
	if calls != 2 || err == nil {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
