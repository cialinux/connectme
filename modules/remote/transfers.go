package remote

import (
	"context"
	"errors"
	"github.com/connectme/connectme/modules/connections"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/pkg/webapi"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"strconv"
	"time"
)

func (m *Module) transferRoutes(r interface {
	Handle(string, string, http.Handler)
}) {
	for _, route := range []struct {
		method, path string
		handler      http.HandlerFunc
	}{
		{"GET", "/api/v1/remote-sessions/{id}/files", m.listFiles},
		{"POST", "/api/v1/remote-sessions/{id}/files/{name}", m.uploadFile},
		{"GET", "/api/v1/remote-sessions/{id}/files/{name}", m.downloadFile},
		{"POST", "/api/v1/remote-sessions/{id}/clipboard-events", m.clipboardEvent},
	} {
		r.Handle(route.method, route.path, m.gate.Protect("connections.manage", route.handler))
	}
}
func (m *Module) operation(w http.ResponseWriter, r *http.Request) (lease, connections.Connection, context.CancelFunc, bool) {
	noop := func() {}
	cookie, err := r.Cookie("connectme_session")
	if err != nil {
		webapi.Problem(w, 401, "session_required", "sessão necessária")
		return lease{}, connections.Connection{}, noop, false
	}
	key := sessionKey(cookie.Value, r.PathValue("id"))
	m.mu.Lock()
	l, ok := m.leases[key]
	m.mu.Unlock()
	if !ok || time.Now().After(l.Expires) {
		webapi.Problem(w, 403, "session_expired", "abra novamente a conexão")
		return l, connections.Connection{}, noop, false
	}
	c, _, _, fp, err := m.validate(r.Context(), l.ConnectionID)
	if err != nil || fp != l.Fingerprint {
		webapi.Problem(w, 403, "session_revoked", "conexão alterada ou bloqueada; reconecte")
		return l, c, noop, false
	}
	deadline := time.Now().Add(2 * time.Minute)
	if l.Expires.Before(deadline) {
		deadline = l.Expires
	}
	ctx, cancel := context.WithDeadline(r.Context(), deadline)
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline)
	user, _ := identity.CurrentUser(r.Context())
	watchDone := make(chan struct{})
	var closed <-chan struct{}
	if l.Files != nil {
		closed = l.Files.done
	}
	go func() {
		defer close(watchDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-closed:
				cancel()
				_ = controller.SetReadDeadline(time.Now())
				_ = controller.SetWriteDeadline(time.Now())
				return
			case <-ticker.C:
				m.mu.Lock()
				_, present := m.leases[key]
				m.mu.Unlock()
				_, authErr := m.identity.Authenticate(ctx, cookie.Value)
				allowed, permErr := m.gate.Allowed(ctx, user.ID, "connections.manage")
				_, _, _, currentFP, e := m.validate(ctx, l.ConnectionID)
				if !present || authErr != nil || permErr != nil || !allowed || e != nil || currentFP != fp {
					cancel()
					_ = controller.SetReadDeadline(time.Now())
					_ = controller.SetWriteDeadline(time.Now())
					return
				}
			}
		}
	}()
	*r = *r.WithContext(ctx)
	w.Header().Set("Cache-Control", "no-store")
	finish := func() {
		cancel()
		<-watchDone
		_ = controller.SetReadDeadline(time.Time{})
		_ = controller.SetWriteDeadline(time.Time{})
	}
	return l, c, finish, true
}
func (m *Module) transferAudit(r *http.Request, action, outcome string, n int64) error {
	user, _ := identity.CurrentUser(r.Context())
	// Use a fresh bounded context for completion records even after cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return m.identity.Audit(ctx, identity.AuditEvent{ActorID: user.ID, Action: action, Outcome: outcome, ResourceType: "remote-session", ResourceID: r.PathValue("id"), Metadata: map[string]any{"bytes": n}})
}
func (m *Module) listFiles(w http.ResponseWriter, r *http.Request) {
	l, c, cancel, ok := m.operation(w, r)
	if !ok {
		return
	}
	defer cancel()
	if l.Files == nil || !c.FileTransferEnabled || (!c.FileUploadEnabled && !c.FileDownloadEnabled) {
		webapi.Problem(w, 403, "files_disabled", "transferência de arquivos desativada")
		return
	}
	items, err := l.Files.list()
	if err != nil {
		webapi.Problem(w, 500, "files_unavailable", "não foi possível listar arquivos")
		return
	}
	webapi.Respond(w, 200, map[string]any{"items": items, "max_file_bytes": maxFileBytes, "session_quota_bytes": sessionFileQuota})
}
func (m *Module) uploadFile(w http.ResponseWriter, r *http.Request) {
	l, c, cancel, ok := m.operation(w, r)
	if !ok {
		return
	}
	defer cancel()
	if l.Files == nil || !c.FileTransferEnabled || !c.FileUploadEnabled {
		webapi.Problem(w, 403, "upload_disabled", "envio de arquivos desativado")
		return
	}
	if !validFilename(r.PathValue("name")) {
		webapi.Problem(w, 400, "filename_invalid", "nome de arquivo inválido")
		return
	}
	if r.ContentLength > maxFileBytes {
		webapi.Problem(w, 413, "file_limit", "limite de 32 MiB por arquivo")
		return
	}
	if err := m.transferAudit(r, "remote.file.upload", "started", 0); err != nil {
		webapi.Problem(w, 503, "audit_unavailable", "auditoria indisponível")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileBytes+1)
	n, err := l.Files.upload(r.PathValue("name"), r.Body)
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	_ = m.transferAudit(r, "remote.file.upload", outcome, n)
	if err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.Is(err, fs.ErrExist):
			webapi.Problem(w, 409, "file_exists", "já existe um arquivo com esse nome; renomeie o envio")
		case errors.Is(err, errTransferLimit) || errors.As(err, &tooLarge):
			webapi.Problem(w, 413, "file_limit", "limite de arquivo ou espaço da sessão excedido")
		default:
			webapi.Problem(w, 400, "upload_failed", "envio cancelado ou armazenamento indisponível")
		}
		return
	}
	webapi.Respond(w, 201, fileEntry{r.PathValue("name"), n})
}
func (m *Module) downloadFile(w http.ResponseWriter, r *http.Request) {
	l, c, cancel, ok := m.operation(w, r)
	if !ok {
		return
	}
	defer cancel()
	if l.Files == nil || !c.FileTransferEnabled || !c.FileDownloadEnabled {
		webapi.Problem(w, 403, "download_disabled", "download desativado")
		return
	}
	f, size, err := l.Files.download(r.PathValue("name"))
	if err != nil {
		code := 404
		if errors.Is(err, errTransferLimit) {
			code = 413
		}
		webapi.Problem(w, code, "file_unavailable", "arquivo indisponível ou maior que 32 MiB")
		return
	}
	defer f.Close()
	if err = m.transferAudit(r, "remote.file.download", "started", 0); err != nil {
		webapi.Problem(w, 503, "audit_unavailable", "auditoria indisponível")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": r.PathValue("name")}))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	n, err := io.CopyN(w, f, size)
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	_ = m.transferAudit(r, "remote.file.download", outcome, n)
}
func (m *Module) clipboardEvent(w http.ResponseWriter, r *http.Request) {
	_, c, cancel, ok := m.operation(w, r)
	if !ok {
		return
	}
	defer cancel()
	var event struct {
		Direction string `json:"direction"`
		Bytes     int64  `json:"bytes"`
	}
	if !webapi.Decode(w, r, &event) {
		return
	}
	if event.Bytes < 0 || event.Bytes > maxClipboardBytes {
		webapi.Problem(w, 413, "clipboard_limit", "limite de 64 KiB de texto")
		return
	}
	allowed := c.ClipboardEnabled && ((event.Direction == "paste" && c.ClipboardPasteEnabled) || (event.Direction == "copy" && c.ClipboardCopyEnabled))
	if !allowed {
		webapi.Problem(w, 403, "clipboard_disabled", "direção da área de transferência desativada")
		return
	}
	if err := m.transferAudit(r, "remote.clipboard."+event.Direction, "authorized", event.Bytes); err != nil {
		webapi.Problem(w, 503, "audit_unavailable", "auditoria indisponível")
		return
	}
	w.WriteHeader(204)
}
