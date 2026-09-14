package system

import (
	"context"
	"encoding/json"
	"github.com/connectme/connectme/core/modules"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"sync/atomic"
	"time"
)

type Module struct {
	pool  *pgxpool.Pool
	ready atomic.Bool
}

func New(pool *pgxpool.Pool) *Module { return &Module{pool: pool} }
func (m *Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "system", Version: "0.1.0", Critical: true}
}
func (m *Module) Register(r modules.Registrar) error {
	r.Handle(http.MethodGet, "/livez", http.HandlerFunc(m.live))
	return nil
}
func (m *Module) Start(context.Context) error { m.ready.Store(true); return nil }
func (m *Module) Stop(context.Context) error  { m.ready.Store(false); return nil }
func (m *Module) Health(ctx context.Context) modules.HealthStatus {
	s := "ok"
	msg := "ready"
	if !m.ready.Load() {
		s = "unavailable"
		msg = "not started"
	} else if err := m.pool.Ping(ctx); err != nil {
		s = "unavailable"
		msg = "database unavailable"
	}
	return modules.HealthStatus{Status: s, Message: msg, CheckedAt: time.Now().UTC()}
}
func (m *Module) live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
