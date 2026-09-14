package authorization

import (
	"context"
	"github.com/connectme/connectme/core/modules"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

const AdministratorRoleID = "00000000-0000-4000-8000-000000000001"

type Service struct{ pool *pgxpool.Pool }

func NewService(p *pgxpool.Pool) *Service { return &Service{p} }
func (s *Service) GrantAdministrator(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, "INSERT INTO authz.user_roles(user_id,role_id) VALUES($1,$2) ON CONFLICT DO NOTHING", userID, AdministratorRoleID)
	return err
}
func (s *Service) Allowed(ctx context.Context, userID, permission string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM authz.user_roles ur JOIN authz.role_permissions rp ON rp.role_id=ur.role_id WHERE ur.user_id=$1 AND rp.permission=$2 UNION SELECT 1 FROM authz.group_members gm JOIN authz.group_roles gr ON gr.group_id=gm.group_id JOIN authz.role_permissions rp ON rp.role_id=gr.role_id WHERE gm.user_id=$1 AND rp.permission=$2)`, userID, permission).Scan(&ok)
	return ok, err
}

type Module struct{ service *Service }

func New(s *Service) *Module { return &Module{s} }
func (m *Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "authorization", Version: "0.1.0", Dependencies: []string{"identity", "audit"}, Critical: true}
}
func (m *Module) Register(modules.Registrar) error { return nil }
func (m *Module) Start(context.Context) error      { return nil }
func (m *Module) Stop(context.Context) error       { return nil }
func (m *Module) Health(context.Context) modules.HealthStatus {
	return modules.HealthStatus{Status: "ok", CheckedAt: time.Now().UTC()}
}
