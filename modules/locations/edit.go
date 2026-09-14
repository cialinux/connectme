package locations

import (
	"github.com/connectme/connectme/pkg/validation"
	"github.com/connectme/connectme/pkg/webapi"
	"net/http"
	"strings"
)

func (m *Module) editRoutes(r interface {
	Handle(string, string, http.Handler)
}) {
	r.Handle("PUT", "/api/v1/locations/{id}", m.p.Protect("locations.manage", http.HandlerFunc(m.update)))
	r.Handle("DELETE", "/api/v1/locations/{id}", m.p.Protect("locations.manage", http.HandlerFunc(m.remove)))
	r.Handle("GET", "/api/v1/networks", m.p.Protect("locations.read", http.HandlerFunc(m.listNetworks)))
	r.Handle("PUT", "/api/v1/networks/{id}", m.p.Protect("locations.manage", http.HandlerFunc(m.updateNetwork)))
	r.Handle("DELETE", "/api/v1/networks/{id}", m.p.Protect("locations.manage", http.HandlerFunc(m.removeNetwork)))
}
func (m *Module) update(w http.ResponseWriter, r *http.Request) {
	var v Location
	if !webapi.Decode(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.Name) == "" {
		webapi.Problem(w, 400, "name_required", "nome obrigatório")
		return
	}
	tag, err := m.s.pool.Exec(r.Context(), "UPDATE locations.locations SET name=$2,region=$3,description=$4,enabled=$5,updated_at=now() WHERE id=$1 AND deleted_at IS NULL", r.PathValue("id"), v.Name, v.Region, v.Description, v.Enabled)
	if err != nil || tag.RowsAffected() != 1 {
		webapi.Problem(w, 400, "update_failed", "não foi possível atualizar localização")
		return
	}
	webapi.Respond(w, 200, map[string]string{"status": "updated"})
}
func (m *Module) remove(w http.ResponseWriter, r *http.Request) {
	tag, err := m.s.pool.Exec(r.Context(), "UPDATE locations.locations SET deleted_at=now(),enabled=false WHERE id=$1 AND deleted_at IS NULL", r.PathValue("id"))
	if err != nil || tag.RowsAffected() != 1 {
		webapi.Problem(w, 404, "missing", "localização não encontrada")
		return
	}
	w.WriteHeader(204)
}
func (m *Module) listNetworks(w http.ResponseWriter, r *http.Request) {
	l, o := webapi.Page(r)
	rows, err := m.s.pool.Query(r.Context(), "SELECT id,location_id,name,cidr::text,enabled FROM locations.networks WHERE deleted_at IS NULL ORDER BY name LIMIT $1 OFFSET $2", l, o)
	if err != nil {
		webapi.Problem(w, 500, "query_failed", "consulta indisponível")
		return
	}
	defer rows.Close()
	items := []Network{}
	for rows.Next() {
		var v Network
		if err = rows.Scan(&v.ID, &v.LocationID, &v.Name, &v.CIDR, &v.Enabled); err != nil {
			webapi.Problem(w, 500, "query_failed", "consulta indisponível")
			return
		}
		items = append(items, v)
	}
	if rows.Err() != nil {
		webapi.Problem(w, 500, "query_failed", "consulta indisponível")
		return
	}
	webapi.Respond(w, 200, map[string]any{"items": items, "limit": l, "offset": o})
}
func (m *Module) updateNetwork(w http.ResponseWriter, r *http.Request) {
	var v Network
	if !webapi.Decode(w, r, &v) {
		return
	}
	cidr, err := validation.CIDR(v.CIDR)
	if err != nil || strings.TrimSpace(v.Name) == "" {
		webapi.Problem(w, 400, "network_invalid", "nome ou CIDR inválido")
		return
	}
	tag, err := m.s.pool.Exec(r.Context(), "UPDATE locations.networks SET name=$2,cidr=$3,enabled=$4,location_id=$5 WHERE id=$1 AND deleted_at IS NULL", r.PathValue("id"), v.Name, cidr.String(), v.Enabled, v.LocationID)
	if err != nil || tag.RowsAffected() != 1 {
		webapi.Problem(w, 400, "update_failed", "não foi possível atualizar rede")
		return
	}
	webapi.Respond(w, 200, map[string]string{"status": "updated"})
}
func (m *Module) removeNetwork(w http.ResponseWriter, r *http.Request) {
	tag, err := m.s.pool.Exec(r.Context(), "UPDATE locations.networks SET deleted_at=now(),enabled=false WHERE id=$1 AND deleted_at IS NULL", r.PathValue("id"))
	if err != nil || tag.RowsAffected() != 1 {
		webapi.Problem(w, 404, "missing", "rede não encontrada")
		return
	}
	w.WriteHeader(204)
}
