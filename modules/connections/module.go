package connections

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/connectme/connectme/core/modules"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/pkg/validation"
	"github.com/connectme/connectme/pkg/webapi"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
	"time"
)

type Protector interface {
	Protect(string, http.Handler) http.Handler
}
type Connection struct {
	ID                    string `json:"id"`
	HostID                string `json:"host_id"`
	CredentialRefID       string `json:"credential_ref_id"`
	Name                  string `json:"name"`
	Protocol              string `json:"protocol"`
	Port                  int    `json:"port"`
	ClipboardEnabled      bool   `json:"clipboard_enabled"`
	FileTransferEnabled   bool   `json:"file_transfer_enabled"`
	ClipboardCopyEnabled  bool   `json:"clipboard_copy_enabled"`
	ClipboardPasteEnabled bool   `json:"clipboard_paste_enabled"`
	FileUploadEnabled     bool   `json:"file_upload_enabled"`
	FileDownloadEnabled   bool   `json:"file_download_enabled"`
	Enabled               bool   `json:"enabled"`
}
type ReferenceSource interface {
	Exists(context.Context, string) (bool, error)
}
type Service struct {
	pool               *pgxpool.Pool
	hosts, credentials ReferenceSource
}

func NewService(p *pgxpool.Pool, h, c ReferenceSource) *Service { return &Service{p, h, c} }
func (s *Service) Create(ctx context.Context, v Connection, actor string) (Connection, error) {
	v.Protocol = strings.ToLower(v.Protocol)
	if v.Protocol != "rdp" && v.Protocol != "ssh" && v.Protocol != "vnc" {
		return Connection{}, errors.New("protocolo inválido")
	}
	if err := validation.Port(v.Port); err != nil {
		return Connection{}, err
	}
	if strings.TrimSpace(v.Name) == "" || v.HostID == "" || v.CredentialRefID == "" {
		return Connection{}, errors.New("nome, host e credencial obrigatórios")
	}
	if s.hosts == nil || s.credentials == nil {
		return Connection{}, errors.New("validadores indisponíveis")
	}
	if ok, err := s.hosts.Exists(ctx, v.HostID); err != nil || !ok {
		return Connection{}, errors.New("host inexistente ou desativado")
	}
	if ok, err := s.credentials.Exists(ctx, v.CredentialRefID); err != nil || !ok {
		return Connection{}, errors.New("credencial inexistente")
	}
	v.ID = uuid()
	v.Enabled = true
	_, err := s.pool.Exec(ctx, "INSERT INTO connections.connections(id,host_id,credential_ref_id,name,protocol,port,clipboard_enabled,file_transfer_enabled,created_by,clipboard_copy_enabled,clipboard_paste_enabled,file_upload_enabled,file_download_enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)", v.ID, v.HostID, v.CredentialRefID, v.Name, v.Protocol, v.Port, v.ClipboardEnabled, v.FileTransferEnabled, actor, v.ClipboardCopyEnabled, v.ClipboardPasteEnabled, v.FileUploadEnabled, v.FileDownloadEnabled)
	return v, err
}
func (s *Service) List(ctx context.Context, limit, offset int) ([]Connection, error) {
	rows, err := s.pool.Query(ctx, "SELECT id,host_id,credential_ref_id,name,protocol,port,clipboard_enabled,file_transfer_enabled,enabled,clipboard_copy_enabled,clipboard_paste_enabled,file_upload_enabled,file_download_enabled FROM connections.connections WHERE deleted_at IS NULL ORDER BY lower(name) LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		var v Connection
		if err = rows.Scan(&v.ID, &v.HostID, &v.CredentialRefID, &v.Name, &v.Protocol, &v.Port, &v.ClipboardEnabled, &v.FileTransferEnabled, &v.Enabled, &v.ClipboardCopyEnabled, &v.ClipboardPasteEnabled, &v.FileUploadEnabled, &v.FileDownloadEnabled); err != nil {
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
	return modules.Descriptor{Name: "connections", Version: "0.2.0", Dependencies: []string{"hosts", "credentials", "authorization", "audit"}}
}
func (m *Module) Register(r modules.Registrar) error {
	m.editRoutes(r)
	r.Handle("GET", "/api/v1/connections", m.p.Protect("connections.read", http.HandlerFunc(m.list)))
	r.Handle("POST", "/api/v1/connections", m.p.Protect("connections.manage", http.HandlerFunc(m.create)))
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
		webapi.Problem(w, 500, "connections_failed", "falha ao consultar conexões")
		return
	}
	webapi.Respond(w, 200, map[string]any{"items": items, "limit": l, "offset": o})
}
func (m *Module) create(w http.ResponseWriter, r *http.Request) {
	var v Connection
	if !webapi.Decode(w, r, &v) {
		return
	}
	u, _ := identity.CurrentUser(r.Context())
	created, err := m.s.Create(r.Context(), v, u.ID)
	if err != nil {
		webapi.Problem(w, 400, "connection_invalid", err.Error())
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
