package credentials

import (
	"context"
	"crypto/rand"
	"github.com/connectme/connectme/pkg/webapi"
	"net/http"
	"strings"
)

func (p *LocalProvider) Metadata(ctx context.Context, id string) (Metadata, error) {
	var v Metadata
	err := p.pool.QueryRow(ctx, "SELECT id,name,type,username,domain,enabled,rotated_at FROM credentials.secret_refs WHERE id=$1 AND deleted_at IS NULL", id).Scan(&v.ID, &v.Name, &v.Type, &v.Username, &v.Domain, &v.Enabled, &v.RotatedAt)
	return v, err
}
func (m *Module) editRoutes(r interface {
	Handle(string, string, http.Handler)
}) {
	r.Handle("PUT", "/api/v1/credentials/{id}", m.p.Protect("credentials.manage", http.HandlerFunc(m.update)))
	r.Handle("DELETE", "/api/v1/credentials/{id}", m.p.Protect("credentials.manage", http.HandlerFunc(m.remove)))
}
func (m *Module) update(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Username string `json:"username"`
		Domain   string `json:"domain"`
		Enabled  bool   `json:"enabled"`
		Value    string `json:"value"`
	}
	if !webapi.Decode(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.Name) == "" || (v.Type != "password" && v.Type != "windows" && v.Type != "ssh-key") {
		webapi.Problem(w, 400, "credential_invalid", "nome ou tipo inválido")
		return
	}
	id := r.PathValue("id")
	tx, err := m.provider.pool.Begin(r.Context())
	if err != nil {
		webapi.Problem(w, 500, "database_error", "operação indisponível")
		return
	}
	defer tx.Rollback(r.Context())
	var oldType string
	if err = tx.QueryRow(r.Context(), "SELECT type FROM credentials.secret_refs WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id).Scan(&oldType); err != nil {
		webapi.Problem(w, 404, "credential_missing", "credencial não encontrada")
		return
	}
	if oldType != v.Type && v.Value == "" {
		webapi.Problem(w, 400, "secret_required", "novo segredo obrigatório ao mudar o tipo")
		return
	}
	if v.Value != "" {
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			webapi.Problem(w, 500, "encryption_error", "gerador seguro indisponível")
			return
		}
		defer clear(key)
		var wrapped, value []byte
		wrapped, err = m.provider.master.Encrypt(key, []byte("dek:"+id+":1"))
		if err == nil {
			value, err = seal(key, []byte(v.Value), []byte("secret:"+id+":"+v.Type))
		}
		if err == nil {
			_, err = tx.Exec(r.Context(), "UPDATE credentials.secret_refs SET ciphertext=$2,encrypted_data_key=$3,key_version=1,rotated_at=now() WHERE id=$1", id, value, wrapped)
		}
		if err != nil {
			webapi.Problem(w, 500, "encryption_error", "falha na rotação")
			return
		}
	}
	_, err = tx.Exec(r.Context(), "UPDATE credentials.secret_refs SET name=$2,type=$3,username=$4,domain=$5,enabled=$6 WHERE id=$1", id, v.Name, v.Type, v.Username, v.Domain, v.Enabled)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		webapi.Problem(w, 400, "update_failed", "não foi possível atualizar")
		return
	}
	webapi.Respond(w, 200, map[string]string{"status": "updated"})
}
func (m *Module) remove(w http.ResponseWriter, r *http.Request) {
	if err := m.provider.Delete(r.Context(), SecretRef{ID: r.PathValue("id")}); err != nil {
		webapi.Problem(w, 400, "delete_failed", "não foi possível remover")
		return
	}
	w.WriteHeader(204)
}
