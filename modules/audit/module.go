package audit

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/connectme/connectme/core/modules"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

type Event struct {
	ActorID, Action, Outcome, ResourceType, ResourceID, CorrelationID, SourceIP string
	Metadata                                                                    map[string]any
}
type Service struct {
	pool *pgxpool.Pool
	key  []byte
}

func NewService(p *pgxpool.Pool, key []byte) *Service {
	return &Service{p, append([]byte(nil), key...)}
}
func (s *Service) Record(ctx context.Context, e Event) error {
	if e.Action == "" || e.Outcome == "" {
		return fmt.Errorf("audit action/outcome obrigatórios")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var previous []byte
	_ = tx.QueryRow(ctx, "SELECT event_hash FROM audit.chain_heads WHERE shard=0 FOR UPDATE").Scan(&previous)
	id := uuid()
	now := time.Now().UTC()
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return err
	}
	canonical, _ := json.Marshal([]any{id, now.Format(time.RFC3339Nano), e.ActorID, e.Action, e.Outcome, e.ResourceType, e.ResourceID, e.CorrelationID, e.SourceIP, json.RawMessage(meta), hex.EncodeToString(previous)})
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(canonical)
	sum := mac.Sum(nil)
	_, err = tx.Exec(ctx, `INSERT INTO audit.events(id,occurred_at,actor_id,action,outcome,resource_type,resource_id,correlation_id,source_ip,metadata,previous_hash,event_hash) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,NULLIF($6,''),NULLIF($7,'')::uuid,NULLIF($8,''),NULLIF($9,'')::inet,$10,$11,$12)`, id, now, e.ActorID, e.Action, e.Outcome, e.ResourceType, e.ResourceID, e.CorrelationID, e.SourceIP, meta, previous, sum)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit.chain_heads(shard,event_id,event_hash) VALUES(0,$1,$2) ON CONFLICT(shard) DO UPDATE SET event_id=excluded.event_id,event_hash=excluded.event_hash`, id, sum)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type Module struct{ service *Service }

func New(s *Service) *Module { return &Module{s} }
func (m *Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "audit", Version: "0.1.0", Dependencies: []string{"system"}, Critical: true}
}
func (m *Module) Register(r modules.Registrar) error {
	r.Handle(http.MethodGet, "/api/v1/audit/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	return nil
}
func (m *Module) Start(context.Context) error { return nil }
func (m *Module) Stop(context.Context) error  { return nil }
func (m *Module) Health(context.Context) modules.HealthStatus {
	return modules.HealthStatus{Status: "ok", CheckedAt: time.Now().UTC()}
}
