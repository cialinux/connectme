package hosts

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/connectme/connectme/core/modules"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/pkg/validation"
	"github.com/connectme/connectme/pkg/webapi"
	"github.com/jackc/pgx/v5/pgxpool"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type Protector interface {
	Protect(string, http.Handler) http.Handler
}
type NetworkSource interface {
	Networks(context.Context, string) ([]netip.Prefix, error)
}
type Host struct {
	ID              string `json:"id"`
	LocationID      string `json:"location_id"`
	Name            string `json:"name"`
	Hostname        string `json:"hostname,omitempty"`
	Address         string `json:"address"`
	PinnedIP        string `json:"pinned_ip"`
	OperatingSystem string `json:"operating_system"`
	Description     string `json:"description"`
	Enabled         bool   `json:"enabled"`
	Status          string `json:"status"`
}
type Service struct {
	pool     *pgxpool.Pool
	networks NetworkSource
	resolver *net.Resolver
}

func NewService(p *pgxpool.Pool, n NetworkSource) *Service {
	return &Service{p, n, net.DefaultResolver}
}
func (s *Service) Exists(ctx context.Context, id string) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM hosts.hosts WHERE id=$1 AND enabled AND deleted_at IS NULL)", id).Scan(&found)
	return found, err
}
func (s *Service) ValidateDestination(ctx context.Context, locationID, address string) (netip.Addr, error) {
	allowed, err := s.networks.Networks(ctx, locationID)
	if err != nil {
		return netip.Addr{}, err
	}
	return validation.Destination(ctx, s.resolver, address, allowed)
}
func (s *Service) Create(ctx context.Context, v Host, actor string) (Host, error) {
	v.Name = strings.TrimSpace(v.Name)
	v.Address = strings.TrimSpace(v.Address)
	v.OperatingSystem = strings.ToLower(v.OperatingSystem)
	if v.Name == "" || v.Address == "" {
		return Host{}, fmt.Errorf("nome e endereço obrigatórios")
	}
	if v.OperatingSystem != "linux" && v.OperatingSystem != "windows" && v.OperatingSystem != "other" {
		return Host{}, fmt.Errorf("sistema operacional inválido")
	}
	ip, err := s.ValidateDestination(ctx, v.LocationID, v.Address)
	if err != nil {
		return Host{}, err
	}
	v.ID = uuid()
	v.PinnedIP = ip.String()
	v.Enabled = true
	v.Status = "unknown"
	_, err = s.pool.Exec(ctx, "INSERT INTO hosts.hosts(id,location_id,name,hostname,address,pinned_ip,operating_system,description,created_by) VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9)", v.ID, v.LocationID, v.Name, v.Hostname, v.Address, v.PinnedIP, v.OperatingSystem, v.Description, actor)
	return v, err
}
func (s *Service) List(ctx context.Context, limit, offset int) ([]Host, error) {
	rows, err := s.pool.Query(ctx, "SELECT id,location_id,name,COALESCE(hostname,''),address,COALESCE(pinned_ip::text,''),operating_system,description,enabled,status FROM hosts.hosts WHERE deleted_at IS NULL ORDER BY lower(name) LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Host{}
	for rows.Next() {
		var v Host
		if err = rows.Scan(&v.ID, &v.LocationID, &v.Name, &v.Hostname, &v.Address, &v.PinnedIP, &v.OperatingSystem, &v.Description, &v.Enabled, &v.Status); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type Module struct {
	s *Service
	p Protector
}

func New(s *Service, p Protector) *Module { return &Module{s, p} }
func (*Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "hosts", Version: "0.2.0", Dependencies: []string{"locations", "authorization", "audit"}}
}
func (m *Module) Register(r modules.Registrar) error {
	m.editRoutes(r)
	r.Handle("GET", "/api/v1/hosts", m.p.Protect("hosts.read", http.HandlerFunc(m.list)))
	r.Handle("POST", "/api/v1/hosts", m.p.Protect("hosts.manage", http.HandlerFunc(m.create)))
	return nil
}
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
func (*Module) Health(context.Context) modules.HealthStatus {
	return modules.HealthStatus{Status: "ok", CheckedAt: time.Now().UTC()}
}
func (m *Module) list(w http.ResponseWriter, r *http.Request) {
	l, o := webapi.Page(r)
	items, err := m.s.List(r.Context(), l, o)
	if err != nil {
		webapi.Problem(w, 500, "hosts_failed", "falha ao consultar hosts")
		return
	}
	webapi.Respond(w, 200, map[string]any{"items": items, "limit": l, "offset": o})
}
func (m *Module) create(w http.ResponseWriter, r *http.Request) {
	var v Host
	if !webapi.Decode(w, r, &v) {
		return
	}
	u, _ := identity.CurrentUser(r.Context())
	created, err := m.s.Create(r.Context(), v, u.ID)
	if err != nil {
		webapi.Problem(w, 400, "host_invalid", err.Error())
		return
	}
	webapi.Respond(w, 201, created)
}
func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
