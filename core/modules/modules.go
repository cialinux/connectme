package modules

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/connectme/connectme/core/events"
)

type HealthStatus struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}
type Descriptor struct {
	Name, Version string
	Dependencies  []string
	Critical      bool
}
type Module interface {
	Descriptor() Descriptor
	Register(Registrar) error
	Start(context.Context) error
	Stop(context.Context) error
	Health(context.Context) HealthStatus
}
type Registrar interface {
	Handle(string, string, http.Handler)
	Events() *events.Bus
}

type Registry struct {
	mu      sync.RWMutex
	modules map[string]Module
	ordered []Module
	health  map[string]HealthStatus
	mux     *http.ServeMux
	bus     *events.Bus
}

func New(mux *http.ServeMux, bus *events.Bus) *Registry {
	return &Registry{modules: map[string]Module{}, health: map[string]HealthStatus{}, mux: mux, bus: bus}
}
func (r *Registry) Add(m Module) error {
	d := m.Descriptor()
	if d.Name == "" || d.Version == "" {
		return errors.New("descriptor inválido")
	}
	if _, exists := r.modules[d.Name]; exists {
		return fmt.Errorf("módulo duplicado: %s", d.Name)
	}
	r.modules[d.Name] = m
	return nil
}
func (r *Registry) Resolve(enabled map[string]bool) error {
	selected := map[string]bool{}
	for name, m := range r.modules {
		on, configured := enabled[name]
		if !configured {
			on = true
		}
		if m.Descriptor().Critical && !on {
			return fmt.Errorf("módulo crítico %s desativado", name)
		}
		selected[name] = on
	}
	state := map[string]int{}
	ordered := []Module{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("dependência circular em %s", name)
		}
		if state[name] == 2 {
			return nil
		}
		m, ok := r.modules[name]
		if !ok {
			return fmt.Errorf("dependência ausente: %s", name)
		}
		if !selected[name] {
			return fmt.Errorf("dependência %s desativada", name)
		}
		state[name] = 1
		deps := append([]string(nil), m.Descriptor().Dependencies...)
		sort.Strings(deps)
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		state[name] = 2
		ordered = append(ordered, m)
		return nil
	}
	names := make([]string, 0, len(r.modules))
	for n, on := range selected {
		if on {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if err := visit(n); err != nil {
			return err
		}
	}
	r.ordered = ordered
	return nil
}
func (r *Registry) Register() error {
	for _, m := range r.ordered {
		if err := m.Register(r); err != nil {
			return fmt.Errorf("register %s: %w", m.Descriptor().Name, err)
		}
	}
	return nil
}
func (r *Registry) Start(ctx context.Context) error {
	for _, m := range r.ordered {
		if err := m.Start(ctx); err != nil {
			return fmt.Errorf("start %s: %w", m.Descriptor().Name, err)
		}
	}
	return nil
}
func (r *Registry) Stop(ctx context.Context) error {
	var errs []error
	for i := len(r.ordered) - 1; i >= 0; i-- {
		m := r.ordered[i]
		if err := m.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop %s: %w", m.Descriptor().Name, err))
		}
	}
	return errors.Join(errs...)
}
func (r *Registry) Check(ctx context.Context) map[string]HealthStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.ordered {
		r.health[m.Descriptor().Name] = m.Health(ctx)
	}
	out := map[string]HealthStatus{}
	for k, v := range r.health {
		out[k] = v
	}
	return out
}
func (r *Registry) Handle(method, pattern string, h http.Handler) {
	r.mux.Handle(method+" "+pattern, h)
}
func (r *Registry) Events() *events.Bus { return r.bus }
func (r *Registry) Descriptors() []Descriptor {
	out := make([]Descriptor, 0, len(r.ordered))
	for _, m := range r.ordered {
		out = append(out, m.Descriptor())
	}
	return out
}
