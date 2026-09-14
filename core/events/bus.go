package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Event struct {
	Name       string
	Version    int
	OccurredAt time.Time
	Payload    any
}
type Handler func(context.Context, Event) error

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func New() *Bus { return &Bus{handlers: make(map[string][]Handler)} }
func (b *Bus) Subscribe(name string, h Handler) error {
	if name == "" || h == nil {
		return errors.New("evento e handler são obrigatórios")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
	return nil
}
func (b *Bus) Publish(ctx context.Context, e Event) error {
	if e.Name == "" || e.Version < 1 {
		return errors.New("evento inválido")
	}
	b.mu.RLock()
	hs := append([]Handler(nil), b.handlers[e.Name]...)
	b.mu.RUnlock()
	var errs []error
	for i, h := range hs {
		if err := h(ctx, e); err != nil {
			errs = append(errs, fmt.Errorf("handler %d: %w", i, err))
		}
	}
	return errors.Join(errs...)
}
