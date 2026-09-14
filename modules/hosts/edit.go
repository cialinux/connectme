package hosts

import (
	"context"
	"errors"
	"github.com/connectme/connectme/pkg/webapi"
	"net/http"
	"strings"
)

func (s *Service) Get(ctx context.Context, id string) (Host, error) {
	var v Host
	err := s.pool.QueryRow(ctx, "SELECT id,location_id,name,COALESCE(hostname,''),address,COALESCE(host(pinned_ip),''),operating_system,description,enabled,status FROM hosts.hosts WHERE id=$1 AND deleted_at IS NULL", id).Scan(&v.ID, &v.LocationID, &v.Name, &v.Hostname, &v.Address, &v.PinnedIP, &v.OperatingSystem, &v.Description, &v.Enabled, &v.Status)
	return v, err
}
func (m *Module) editRoutes(r interface {
	Handle(string, string, http.Handler)
}) {
	r.Handle("PUT", "/api/v1/hosts/{id}", m.p.Protect("hosts.manage", http.HandlerFunc(m.update)))
	r.Handle("DELETE", "/api/v1/hosts/{id}", m.p.Protect("hosts.manage", http.HandlerFunc(m.remove)))
}
func (s *Service) Update(ctx context.Context, id string, v Host) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("nome obrigatório")
	}
	if v.OperatingSystem != "linux" && v.OperatingSystem != "windows" && v.OperatingSystem != "other" {
		return errors.New("sistema operacional inválido")
	}
	old, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	pinned := old.PinnedIP
	if v.Enabled {
		ip, e := s.ValidateDestination(ctx, v.LocationID, v.Address)
		if e != nil {
			return e
		}
		pinned = ip.String()
	}
	tag, err := s.pool.Exec(ctx, "UPDATE hosts.hosts SET name=$2,location_id=$3,hostname=$4,address=$5,pinned_ip=$6,operating_system=$7,description=$8,enabled=$9,updated_at=now() WHERE id=$1 AND deleted_at IS NULL", id, v.Name, v.LocationID, v.Hostname, v.Address, pinned, v.OperatingSystem, v.Description, v.Enabled)
	if err == nil && tag.RowsAffected() != 1 {
		return errors.New("host não encontrado")
	}
	return err
}
func (m *Module) update(w http.ResponseWriter, r *http.Request) {
	var v Host
	if !webapi.Decode(w, r, &v) {
		return
	}
	if err := m.s.Update(r.Context(), r.PathValue("id"), v); err != nil {
		webapi.Problem(w, 400, "host_invalid", err.Error())
		return
	}
	webapi.Respond(w, 200, map[string]string{"status": "updated"})
}
func (m *Module) remove(w http.ResponseWriter, r *http.Request) {
	tag, err := m.s.pool.Exec(r.Context(), "UPDATE hosts.hosts SET enabled=false,deleted_at=now(),updated_at=now() WHERE id=$1 AND deleted_at IS NULL", r.PathValue("id"))
	if err != nil || tag.RowsAffected() != 1 {
		webapi.Problem(w, 404, "host_missing", "host não encontrado")
		return
	}
	w.WriteHeader(204)
}
