package locations

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/connectme/connectme/core/modules"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/pkg/validation"
	"github.com/connectme/connectme/pkg/webapi"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type Protector interface {
	Protect(string, http.Handler) http.Handler
}
type Auditor interface {
	Record(context.Context, identity.AuditEvent) error
}
type Location struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Region      string    `json:"region"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}
type Network struct {
	ID         string `json:"id"`
	LocationID string `json:"location_id"`
	Name       string `json:"name"`
	CIDR       string `json:"cidr"`
	Enabled    bool   `json:"enabled"`
}
type Service struct{ pool *pgxpool.Pool }

func NewService(p *pgxpool.Pool) *Service { return &Service{p} }
func (s *Service) Create(ctx context.Context, name, region, description, actor string) (Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Location{}, fmt.Errorf("nome obrigatório")
	}
	v := Location{ID: uuid(), Name: name, Region: strings.TrimSpace(region), Description: strings.TrimSpace(description), Enabled: true}
	err := s.pool.QueryRow(ctx, "INSERT INTO locations.locations(id,name,region,description,created_by) VALUES($1,$2,$3,$4,$5) RETURNING created_at", v.ID, v.Name, v.Region, v.Description, actor).Scan(&v.CreatedAt)
	return v, err
}
func (s *Service) List(ctx context.Context, limit, offset int) ([]Location, error) {
	rows, err := s.pool.Query(ctx, "SELECT id,name,region,description,enabled,created_at FROM locations.locations WHERE deleted_at IS NULL ORDER BY lower(name) LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Location{}
	for rows.Next() {
		var v Location
		if err = rows.Scan(&v.ID, &v.Name, &v.Region, &v.Description, &v.Enabled, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) AddNetwork(ctx context.Context, locationID, name, cidr string) (Network, error) {
	p, err := validation.CIDR(cidr)
	if err != nil {
		return Network{}, err
	}
	v := Network{ID: uuid(), LocationID: locationID, Name: strings.TrimSpace(name), CIDR: p.String(), Enabled: true}
	if v.Name == "" {
		return Network{}, fmt.Errorf("nome obrigatório")
	}
	_, err = s.pool.Exec(ctx, "INSERT INTO locations.networks(id,location_id,name,cidr) VALUES($1,$2,$3,$4)", v.ID, v.LocationID, v.Name, v.CIDR)
	return v, err
}
func (s *Service) Networks(ctx context.Context, locationID string) ([]netip.Prefix, error) {
	rows, err := s.pool.Query(ctx, "SELECT n.cidr::text FROM locations.networks n JOIN locations.locations l ON l.id=n.location_id WHERE n.location_id=$1 AND n.enabled AND n.deleted_at IS NULL AND l.enabled AND l.deleted_at IS NULL", locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []netip.Prefix
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, err
		}
		p, _ := netip.ParsePrefix(v)
		out = append(out, p)
	}
	return out, rows.Err()
}

type Module struct {
	s *Service
	p Protector
}

func New(s *Service, p Protector) *Module { return &Module{s, p} }
func (*Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "locations", Version: "0.2.0", Dependencies: []string{"authorization", "audit"}}
}
func (m *Module) Register(r modules.Registrar) error {
	m.editRoutes(r)
	r.Handle("GET", "/api/v1/locations", m.p.Protect("locations.read", http.HandlerFunc(m.list)))
	r.Handle("POST", "/api/v1/locations", m.p.Protect("locations.manage", http.HandlerFunc(m.create)))
	r.Handle("POST", "/api/v1/locations/{id}/networks", m.p.Protect("locations.manage", http.HandlerFunc(m.addNetwork)))
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
		webapi.Problem(w, 500, "locations_failed", "falha ao consultar localizações")
		return
	}
	webapi.Respond(w, 200, map[string]any{"items": items, "limit": l, "offset": o})
}
func (m *Module) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Region, Description string }
	if !webapi.Decode(w, r, &in) {
		return
	}
	u, _ := identity.CurrentUser(r.Context())
	v, err := m.s.Create(r.Context(), in.Name, in.Region, in.Description, u.ID)
	if err != nil {
		webapi.Problem(w, 400, "location_invalid", err.Error())
		return
	}
	webapi.Respond(w, 201, v)
}
func (m *Module) addNetwork(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, CIDR string }
	if !webapi.Decode(w, r, &in) {
		return
	}
	v, err := m.s.AddNetwork(r.Context(), r.PathValue("id"), in.Name, in.CIDR)
	if err != nil {
		webapi.Problem(w, 400, "network_invalid", err.Error())
		return
	}
	webapi.Respond(w, 201, v)
}
func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
