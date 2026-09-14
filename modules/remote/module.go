// Package remote integrates the official Guacamole web application for LAN tests.
package remote

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/connectme/connectme/core/modules"
	"github.com/connectme/connectme/modules/connections"
	"github.com/connectme/connectme/modules/credentials"
	"github.com/connectme/connectme/modules/hosts"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/pkg/webapi"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Gate interface {
	Protect(string, http.Handler) http.Handler
	Allowed(context.Context, string, string) (bool, error)
}
type lease struct {
	ID, Name, RemoteID string
	Owner              [32]byte
	ConnectionID       string
	Expires            time.Time
	Fingerprint        [32]byte
	Token              string
	Active             bool
	Files              *fileSpace
}

type Module struct {
	identity    *identity.Service
	gate        Gate
	hosts       *hosts.Service
	creds       *credentials.LocalProvider
	connections *connections.Service
	key         []byte
	proxy       *httputil.ReverseProxy
	mu          sync.Mutex
	leases      map[[32]byte]lease
	store       *fileStore
	stop        chan struct{}
	stopOnce    sync.Once
}

func New(i *identity.Service, g Gate, h *hosts.Service, c *credentials.LocalProvider, s *connections.Service) (*Module, error) {
	key, err := hex.DecodeString(os.Getenv("CONNECTME_GUACAMOLE_KEY"))
	if err != nil || len(key) != 16 {
		return nil, errors.New("CONNECTME_GUACAMOLE_KEY deve conter 16 bytes em hexadecimal")
	}
	u, err := url.Parse("http://guacamole:8080")
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		webapi.Problem(w, 502, "remote_unavailable", "Guacamole indisponível")
	}
	old := proxy.Director
	proxy.Director = func(r *http.Request) {
		old(r)
		r.Header.Del("Cookie")
		r.Header.Del("Authorization")
		r.Header.Del("X-CSRF-Token")
		r.Host = u.Host
	}
	store, err := newFileStore(os.Getenv("CONNECTME_TRANSFER_ROOT"))
	if err != nil {
		return nil, err
	}
	m := &Module{identity: i, gate: g, hosts: h, creds: c, connections: s, key: key, proxy: proxy, leases: map[[32]byte]lease{}, store: store, stop: make(chan struct{})}
	return m, nil
}

func (*Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "remote-desktop", Version: "0.1.0", Dependencies: []string{"connections", "identity", "hosts", "credentials"}}
}
func (m *Module) Register(r modules.Registrar) error {
	m.transferRoutes(r)
	r.Handle("POST", "/api/v1/connections/{id}/open", m.gate.Protect("connections.manage", http.HandlerFunc(m.launch)))
	r.Handle("GET", "/remote/{id}/websocket", http.HandlerFunc(m.serve))
	r.Handle("DELETE", "/api/v1/remote-sessions/{id}", m.gate.Protect("connections.manage", http.HandlerFunc(m.closeSession)))
	r.Handle("GET", "/remote-assets/guacamole.js", m.gate.Protect("connections.manage", http.HandlerFunc(m.clientLibrary)))
	r.Handle("GET", "/guacamole/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/app", http.StatusSeeOther) }))
	return nil
}
func (m *Module) Start(context.Context) error {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.stop:
				return
			case <-ticker.C:
				m.expireSessions()
			}
		}
	}()
	return nil
}
func (m *Module) Stop(context.Context) error {
	m.stopOnce.Do(func() { close(m.stop) })
	m.mu.Lock()
	leases := m.leases
	m.leases = map[[32]byte]lease{}
	m.mu.Unlock()
	for _, l := range leases {
		l.Files.close()
	}
	return nil
}
func (*Module) Health(ctx context.Context) modules.HealthStatus {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://guacamole:8080/guacamole/", nil)
	res, err := http.DefaultClient.Do(request)
	state := modules.HealthStatus{Status: "ok", Message: "Guacamole reachable; destination RDP not probed", CheckedAt: time.Now().UTC()}
	if err != nil {
		state.Status = "error"
		state.Message = "Guacamole unavailable"
		return state
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		state.Status = "error"
		state.Message = "Guacamole not ready"
	}
	return state
}
func (m *Module) validate(ctx context.Context, id string) (connections.Connection, hosts.Host, credentials.Metadata, [32]byte, error) {
	var fingerprint [32]byte
	c, err := m.connections.Get(ctx, id)
	if err != nil || !c.Enabled {
		return c, hosts.Host{}, credentials.Metadata{}, fingerprint, errors.New("conexão indisponível")
	}
	h, err := m.hosts.Get(ctx, c.HostID)
	if err != nil || !h.Enabled {
		return c, h, credentials.Metadata{}, fingerprint, errors.New("host indisponível")
	}
	ip, err := m.hosts.ValidateDestination(ctx, h.LocationID, h.Address)
	if err != nil {
		return c, h, credentials.Metadata{}, fingerprint, err
	}
	if ip.String() != h.PinnedIP {
		return c, h, credentials.Metadata{}, fingerprint, errors.New("endereço mudou: edite e valide o host novamente")
	}
	if !destinationAllowed(ip, c.Protocol, c.Port) {
		return c, h, credentials.Metadata{}, fingerprint, errors.New("destino interno bloqueado")
	}
	meta, err := m.creds.Metadata(ctx, c.CredentialRefID)
	if err != nil || !meta.Enabled {
		return c, h, meta, fingerprint, errors.New("credencial indisponível")
	}
	var hostKey string
	if c.Protocol == "ssh" {
		hostKey, err = sshHostKey(h.PinnedIP, c.Port)
		if err != nil {
			return c, h, meta, fingerprint, err
		}
	}
	b, _ := json.Marshal([]any{c, h, meta, hostKey})
	return c, h, meta, sha256.Sum256(b), nil
}

// signedData follows guacamole-auth-json's documented wire format, not a new cipher.
func signedData(key []byte, value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(b)
	plain := append(mac.Sum(nil), b...)
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	for j := 0; j < padding; j++ {
		plain = append(plain, byte(padding))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(out, plain)
	return base64.StdEncoding.EncodeToString(out), nil
}
func (m *Module) launch(w http.ResponseWriter, r *http.Request) {
	c, h, meta, fp, err := m.validate(r.Context(), r.PathValue("id"))
	if err != nil {
		webapi.Problem(w, 400, "remote_invalid", err.Error())
		return
	}
	if c.Protocol != "rdp" && c.Protocol != "ssh" {
		webapi.Problem(w, 400, "protocol_pending", "execução VNC ainda indisponível")
		return
	}
	if c.Protocol == "rdp" && meta.Type == "ssh-key" {
		webapi.Problem(w, 400, "credential_type", "RDP exige credencial de senha Windows/password")
		return
	}
	if c.Protocol != "vnc" && meta.Username == "" {
		webapi.Problem(w, 400, "username_required", "edite a credencial e informe o utilizador remoto")
		return
	}
	secret, err := m.creds.Retrieve(r.Context(), credentials.SecretRef{ID: meta.ID})
	if err != nil {
		webapi.Problem(w, 500, "secret_unavailable", "não foi possível decifrar a credencial")
		return
	}
	defer clear(secret.Value)
	parameters := map[string]string{"hostname": h.PinnedIP, "port": strconv.Itoa(c.Port), "username": meta.Username, "domain": meta.Domain, "password": string(secret.Value), "security": "nla", "client-name": "ConnectMe", "disable-copy": strconv.FormatBool(!c.ClipboardEnabled || !c.ClipboardCopyEnabled), "disable-paste": strconv.FormatBool(!c.ClipboardEnabled || !c.ClipboardPasteEnabled)}
	if c.Protocol == "rdp" {
		parameters["ignore-cert"] = strconv.FormatBool(allowUntrustedCertificate(h.PinnedIP))
	} else {
		hostKey, keyErr := sshHostKey(h.PinnedIP, c.Port)
		if keyErr != nil {
			webapi.Problem(w, 400, "ssh_host_key", keyErr.Error())
			return
		}
		parameters = map[string]string{"hostname": h.PinnedIP, "port": strconv.Itoa(c.Port), "username": meta.Username, "password": string(secret.Value), "host-key": hostKey, "font-name": "monospace", "font-size": "14", "color-scheme": "gray-black", "enable-sftp": "false", "disable-upload": "true", "disable-download": "true", "disable-copy": strconv.FormatBool(!c.ClipboardEnabled || !c.ClipboardCopyEnabled), "disable-paste": strconv.FormatBool(!c.ClipboardEnabled || !c.ClipboardPasteEnabled)}
	}
	if meta.Type == "ssh-key" {
		delete(parameters, "password")
		parameters["private-key"] = string(secret.Value)
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		webapi.Problem(w, 500, "random_error", "gerador seguro indisponível")
		return
	}
	id := hex.EncodeToString(random[:])
	var files *fileSpace
	retained := false
	defer func() {
		if !retained {
			files.close()
		}
	}()
	if c.Protocol == "rdp" && c.FileTransferEnabled && (c.FileUploadEnabled || c.FileDownloadEnabled) {
		files, err = m.store.create(id)
		if err != nil {
			webapi.Problem(w, 503, "files_unavailable", "armazenamento temporário indisponível; verifique o volume de transferências")
			return
		}
		parameters["enable-drive"] = "true"
		parameters["drive-name"] = "ConnectMe"
		parameters["drive-path"] = files.root.Name()
		parameters["create-drive-path"] = "false"
		// Files move only via the bounded, audited HTTP API, not Guacamole streams.
		parameters["disable-upload"] = "true"
		parameters["disable-download"] = "true"
	}
	u, _ := identity.CurrentUser(r.Context())
	expires := time.Now().Add(30 * time.Minute)
	data, err := signedData(m.key, map[string]any{"username": u.ID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10), "expires": time.Now().Add(time.Minute).UnixMilli(), "connections": map[string]any{c.Name: map[string]any{"protocol": c.Protocol, "parameters": parameters}}})
	if err != nil {
		webapi.Problem(w, 500, "remote_error", "falha na autorização remota")
		return
	}
	cookie, err := r.Cookie("connectme_session")
	if err != nil {
		webapi.Problem(w, 401, "unauthorized", "sessão necessária")
		return
	}

	token, remoteID, err := m.authorize(r.Context(), data, c.Name)
	if err != nil {
		webapi.Problem(w, 502, "remote_unavailable", "não foi possível autorizar no gateway remoto")
		return
	}
	owner := sha256.Sum256([]byte(cookie.Value))
	key := sessionKey(cookie.Value, id)
	m.expireSessions()
	m.mu.Lock()
	count := 0
	for _, v := range m.leases {
		if v.Owner == owner {
			count++
		}
	}
	if count >= 8 {
		m.mu.Unlock()
		m.revoke(token)
		webapi.Problem(w, 429, "session_limit", "limite de 8 sessões; encerre uma antes de abrir outra")
		return
	}
	m.leases[key] = lease{ID: id, Name: c.Name, RemoteID: remoteID, Owner: owner, ConnectionID: c.ID, Expires: expires, Fingerprint: fp, Token: token, Files: files}
	m.mu.Unlock()
	retained = true
	w.Header().Set("Cache-Control", "no-store")
	webapi.Respond(w, 200, map[string]any{"id": id, "name": c.Name, "tunnel": "/remote/" + id + "/websocket", "expires_at": expires, "capabilities": map[string]bool{"copy": c.ClipboardEnabled && c.ClipboardCopyEnabled, "paste": c.ClipboardEnabled && c.ClipboardPasteEnabled, "upload": files != nil && c.FileUploadEnabled, "download": files != nil && c.FileDownloadEnabled}})

}
func (m *Module) serve(w http.ResponseWriter, r *http.Request) {
	// This adapter requires WebSocket: legacy HTTP tunnel IDs are not bearer-bound.
	if r.URL.Path == "/guacamole/tunnel" {
		http.Error(w, "Use WebSocket para o acesso remoto", 403)
		return
	}
	cookie, err := r.Cookie("connectme_session")
	if err != nil {
		http.Error(w, "Sessão necessária", 401)
		return
	}
	u, err := m.identity.Authenticate(r.Context(), cookie.Value)
	if err != nil || u.MustChangePassword {
		http.Error(w, "Sessão inválida", 401)
		return
	}
	allowed, e := m.gate.Allowed(r.Context(), u.ID, "connections.manage")
	if e != nil || !allowed {
		http.Error(w, "Permissão revogada", 403)
		return
	}
	if !validOrigin(r) {
		http.Error(w, "Origem inválida", 403)
		return
	}
	key := sessionKey(cookie.Value, r.PathValue("id"))
	m.mu.Lock()
	l, ok := m.leases[key]
	m.mu.Unlock()
	if !ok || time.Now().After(l.Expires) {
		http.Error(w, "Abra a conexão pelo painel", 403)
		return
	}
	_, _, _, fp, err := m.validate(r.Context(), l.ConnectionID)
	if err != nil || fp != l.Fingerprint {
		http.Error(w, "Conexão alterada ou bloqueada; abra novamente", 403)
		return
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "WebSocket obrigatório", 400)
		return
	}
	m.mu.Lock()
	current, present := m.leases[key]
	if !present || current.Active || current.Expires != l.Expires {
		m.mu.Unlock()
		http.Error(w, "Sessão já utilizada ou encerrada", 409)
		return
	}
	current.Active = true
	m.leases[key] = current
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		if current, ok := m.leases[key]; ok {
			current.Active = false
			m.leases[key] = current
		}
		m.mu.Unlock()
	}()
	ctx, cancel := context.WithDeadline(r.Context(), l.Expires)
	defer cancel()
	// Close long-running tunnels when the session or resources are revoked/edited.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.Lock()
				current, present := m.leases[key]
				m.mu.Unlock()
				_, err := m.identity.Authenticate(ctx, cookie.Value)
				allowed, authErr := m.gate.Allowed(ctx, u.ID, "connections.manage")
				_, _, _, nowFP, e := m.validate(ctx, l.ConnectionID)
				if !present || current.Expires != l.Expires || !allowed || authErr != nil || err != nil || e != nil || nowFP != fp {
					cancel()
					return
				}
			}
		}
	}()
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(l.Expires)
	_ = controller.SetReadDeadline(l.Expires)

	w.Header().Set("Cache-Control", "no-store")
	q := url.Values{"token": {l.Token}, "GUAC_DATA_SOURCE": {"json"}, "GUAC_ID": {l.RemoteID}, "GUAC_TYPE": {"c"}}
	for _, name := range []string{"GUAC_WIDTH", "GUAC_HEIGHT", "GUAC_DPI"} {
		if v, e := strconv.Atoi(r.URL.Query().Get(name)); e == nil && v > 0 && v <= 8192 {
			q.Set(name, strconv.Itoa(v))
		}
	}
	r.URL.Path = "/guacamole/websocket-tunnel"
	r.URL.RawPath = ""
	r.URL.RawQuery = q.Encode()
	m.proxy.ServeHTTP(w, r.WithContext(ctx))
}
