package connections

import (
	"context"
	"github.com/connectme/connectme/pkg/validation"
	"github.com/connectme/connectme/pkg/webapi"
	"net/http"
	"strings"
)

func (s *Service) Get(ctx context.Context, id string) (Connection, error) {
	var v Connection
	err := s.pool.QueryRow(ctx, "SELECT id,host_id,credential_ref_id,name,protocol,port,clipboard_enabled,file_transfer_enabled,enabled,clipboard_copy_enabled,clipboard_paste_enabled,file_upload_enabled,file_download_enabled FROM connections.connections WHERE id=$1 AND deleted_at IS NULL", id).Scan(&v.ID, &v.HostID, &v.CredentialRefID, &v.Name, &v.Protocol, &v.Port, &v.ClipboardEnabled, &v.FileTransferEnabled, &v.Enabled, &v.ClipboardCopyEnabled, &v.ClipboardPasteEnabled, &v.FileUploadEnabled, &v.FileDownloadEnabled)
	return v, err
}
func (m *Module) editRoutes(r interface {
	Handle(string, string, http.Handler)
}) {
	r.Handle("PUT", "/api/v1/connections/{id}", m.p.Protect("connections.manage", http.HandlerFunc(m.update)))
	r.Handle("DELETE", "/api/v1/connections/{id}", m.p.Protect("connections.manage", http.HandlerFunc(m.remove)))
}
func (m *Module) update(w http.ResponseWriter, r *http.Request) {
	var v Connection
	if !webapi.Decode(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.Name) == "" || validation.Port(v.Port) != nil || (v.Protocol != "rdp" && v.Protocol != "ssh" && v.Protocol != "vnc") {
		webapi.Problem(w, 400, "connection_invalid", "nome, porta ou protocolo inválido")
		return
	}
	if ok, err := m.s.hosts.Exists(r.Context(), v.HostID); v.Enabled && (err != nil || !ok) {
		webapi.Problem(w, 400, "host_invalid", "host inexistente ou bloqueado")
		return
	}
	if ok, err := m.s.credentials.Exists(r.Context(), v.CredentialRefID); v.Enabled && (err != nil || !ok) {
		webapi.Problem(w, 400, "credential_invalid", "credencial inexistente ou bloqueada")
		return
	}
	tag, err := m.s.pool.Exec(r.Context(), "UPDATE connections.connections SET name=$2,host_id=$3,credential_ref_id=$4,protocol=$5,port=$6,clipboard_enabled=$7,file_transfer_enabled=$8,enabled=$9,clipboard_copy_enabled=$10,clipboard_paste_enabled=$11,file_upload_enabled=$12,file_download_enabled=$13,updated_at=now() WHERE id=$1 AND deleted_at IS NULL", r.PathValue("id"), v.Name, v.HostID, v.CredentialRefID, v.Protocol, v.Port, v.ClipboardEnabled, v.FileTransferEnabled, v.Enabled, v.ClipboardCopyEnabled, v.ClipboardPasteEnabled, v.FileUploadEnabled, v.FileDownloadEnabled)
	if err != nil || tag.RowsAffected() != 1 {
		webapi.Problem(w, 400, "connection_update_failed", "não foi possível atualizar")
		return
	}
	webapi.Respond(w, 200, map[string]string{"status": "updated"})
}
func (m *Module) remove(w http.ResponseWriter, r *http.Request) {
	tag, err := m.s.pool.Exec(r.Context(), "UPDATE connections.connections SET enabled=false,deleted_at=now(),updated_at=now() WHERE id=$1 AND deleted_at IS NULL", r.PathValue("id"))
	if err != nil || tag.RowsAffected() != 1 {
		webapi.Problem(w, 404, "connection_missing", "conexão não encontrada")
		return
	}
	w.WriteHeader(204)
}
