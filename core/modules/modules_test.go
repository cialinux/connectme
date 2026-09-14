package modules

import (
	"context"
	"github.com/connectme/connectme/core/events"
	"net/http"
	"testing"
	"time"
)

type fake struct {
	d      Descriptor
	starts *[]string
}

func (f fake) Descriptor() Descriptor      { return f.d }
func (f fake) Register(Registrar) error    { return nil }
func (f fake) Start(context.Context) error { *f.starts = append(*f.starts, f.d.Name); return nil }
func (f fake) Stop(context.Context) error  { return nil }
func (f fake) Health(context.Context) HealthStatus {
	return HealthStatus{Status: "ok", CheckedAt: time.Now()}
}

func TestRegistryOrdersDependencies(t *testing.T) {
	starts := []string{}
	r := New(http.NewServeMux(), events.New())
	_ = r.Add(fake{Descriptor{Name: "consumer", Version: "1", Dependencies: []string{"provider"}}, &starts})
	_ = r.Add(fake{Descriptor{Name: "provider", Version: "1"}, &starts})
	if err := r.Resolve(nil); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(starts) != 2 || starts[0] != "provider" {
		t.Fatalf("ordem inválida: %v", starts)
	}
}
func TestRegistryRejectsCycle(t *testing.T) {
	r := New(http.NewServeMux(), events.New())
	s := []string{}
	_ = r.Add(fake{Descriptor{Name: "a", Version: "1", Dependencies: []string{"b"}}, &s})
	_ = r.Add(fake{Descriptor{Name: "b", Version: "1", Dependencies: []string{"a"}}, &s})
	if err := r.Resolve(nil); err == nil {
		t.Fatal("esperava ciclo")
	}
}
