package identity

import (
	"github.com/connectme/connectme/pkg/webapi"
	"net/http"
	"strings"
)

func (m *Module) editUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := CurrentUser(r.Context())
	id := r.PathValue("id")
	var in struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		Enabled     bool   `json:"enabled"`
	}
	removing := r.Method == http.MethodDelete
	if !removing && !webapi.Decode(w, r, &in) {
		return
	}
	if id == actor.ID && (removing || !in.Enabled || in.Password != "") {
		problem(w, 400, "self_protection", "não bloqueie/remova a própria conta; altere sua senha no formulário específico")
		return
	}
	if !removing && (strings.TrimSpace(in.Email) == "" || strings.TrimSpace(in.DisplayName) == "") {
		problem(w, 400, "invalid_user", "utilizador e nome obrigatórios")
		return
	}
	var hash string
	var err error
	if in.Password != "" {
		hash, err = HashPassword(in.Password)
		if err != nil {
			problem(w, 400, "invalid_password", err.Error())
			return
		}
	}
	tx, err := m.service.pool.Begin(r.Context())
	if err != nil {
		problem(w, 500, "database_error", "operação indisponível")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock($1)", int64(0x434d55534552))
	var active bool
	if err == nil {
		err = tx.QueryRow(r.Context(), "SELECT enabled AND deleted_at IS NULL FROM identity.users WHERE id=$1", actor.ID).Scan(&active)
	}
	if err != nil || !active {
		problem(w, 403, "actor_inactive", "conta indisponível")
		return
	}
	query := "UPDATE identity.users SET email=$2,display_name=$3,enabled=$4,updated_at=now() WHERE id=$1 AND deleted_at IS NULL"
	args := []any{id, strings.TrimSpace(in.Email), strings.TrimSpace(in.DisplayName), in.Enabled}
	if removing {
		query = "UPDATE identity.users SET enabled=false,deleted_at=now(),updated_at=now() WHERE id=$1 AND deleted_at IS NULL"
		args = []any{id}
	}
	tag, err := tx.Exec(r.Context(), query, args...)
	if err != nil || tag.RowsAffected() != 1 {
		problem(w, 400, "update_failed", "utilizador não encontrado ou nome em uso")
		return
	}
	if hash != "" {
		_, err = tx.Exec(r.Context(), "UPDATE identity.password_credentials SET password_hash=$2,changed_at=now() WHERE user_id=$1", id, hash)
		if err == nil {
			_, err = tx.Exec(r.Context(), "UPDATE identity.users SET must_change_password=true WHERE id=$1", id)
		}
	}
	if err == nil && (removing || !in.Enabled || hash != "") {
		_, err = tx.Exec(r.Context(), "UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL", id)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		problem(w, 500, "update_failed", "operação indisponível")
		return
	}
	m.service.record(r.Context(), AuditEvent{ActorID: actor.ID, Action: "identity.user.updated", Outcome: "success", ResourceType: "user", ResourceID: id, Metadata: map[string]any{"removed": removing, "enabled": in.Enabled, "password_reset": hash != ""}})
	if removing {
		w.WriteHeader(204)
		return
	}
	respond(w, 200, map[string]string{"status": "updated"})
}
