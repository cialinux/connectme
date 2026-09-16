package remote

import (
	"context"
	"errors"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/pkg/webapi"
	"golang.org/x/crypto/ssh"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Only a key exchange. No password, private key or user authentication is sent.
func probeSSHKey(ctx context.Context, address string, port int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	endpoint := net.JoinHostPort(address, strconv.Itoa(port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return "", errors.New("servidor SSH inacessível; verifique endereço, porta e rede")
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	captured := errors.New("host key captured before authentication")
	var key string
	// Match the non-certificate preference of libssh2 in the official guacd
	// image, so discovery and the gateway select the same server identity.
	cfg := &ssh.ClientConfig{HostKeyAlgorithms: []string{ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521, ssh.KeyAlgoED25519, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}, User: "connectme-identity-check", HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
		key = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
		return captured
	}}
	client, _, _, err := ssh.NewClientConn(conn, endpoint, cfg)
	if client != nil {
		client.Close()
	}
	if !errors.Is(err, captured) || key == "" {
		return "", errors.New("não foi possível verificar a identidade SSH antes da autenticação")
	}
	return key, nil
}
func sshFingerprint(key string) string {
	parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		return ""
	}
	return ssh.FingerprintSHA256(parsed)
}
func (m *Module) storedSSHHostKey(ctx context.Context, id, address string, port int) (string, error) {
	key, err := m.connections.SSHKey(ctx, id, address, port)
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", errors.New("registre a identidade SSH pelo painel antes de conectar")
	}
	if sshFingerprint(key) == "" {
		return "", errors.New("identidade SSH armazenada inválida")
	}
	return "[" + address + "]:" + strconv.Itoa(port) + " " + key, nil
}
func (m *Module) sshIdentity(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Approve  string `json:"approve"`
		Previous string `json:"previous"`
	}
	if !webapi.Decode(w, r, &in) {
		return
	}
	c, h, _, _, err := m.validateWithTrust(r.Context(), r.PathValue("id"), false)
	if err != nil || c.Protocol != "ssh" {
		webapi.Problem(w, 400, "ssh_invalid", "conexão SSH indisponível ou destino não autorizado")
		return
	}
	previous, err := m.connections.SSHKey(r.Context(), c.ID, h.PinnedIP, c.Port)
	if err != nil {
		webapi.Problem(w, 500, "ssh_storage", "não foi possível ler identidade SSH")
		return
	}
	observed, err := probeSSHKey(r.Context(), h.PinnedIP, c.Port)
	if err != nil {
		webapi.Problem(w, 502, "ssh_probe", err.Error())
		return
	}
	expected := previous
	changed := expected != "" && expected != observed
	if changed && (in.Approve != sshFingerprint(observed) || in.Previous != sshFingerprint(expected)) {
		webapi.Respond(w, 200, map[string]any{"changed": true, "previous": sshFingerprint(expected), "fingerprint": sshFingerprint(observed)})
		return
	}
	// Recheck host configuration after network I/O; never save trust for stale UI.
	_, again, _, _, e := m.validateWithTrust(r.Context(), c.ID, false)
	current, e2 := m.connections.Get(r.Context(), c.ID)
	if e != nil || e2 != nil || again.PinnedIP != h.PinnedIP || current.Port != c.Port || current.HostID != c.HostID {
		webapi.Problem(w, 409, "ssh_changed", "destino alterado; tente novamente")
		return
	}
	if previous != observed {
		actor, ok := identity.CurrentUser(r.Context())
		if !ok {
			webapi.Problem(w, 401, "auth", "sessão necessária")
			return
		}
		if err = m.connections.SaveSSHKey(r.Context(), c.ID, h.PinnedIP, c.Port, previous, observed, actor.ID); err != nil {
			webapi.Problem(w, 409, "ssh_conflict", "identidade alterada simultaneamente; tente novamente")
			return
		}
	}
	webapi.Respond(w, 200, map[string]any{"changed": false, "fingerprint": sshFingerprint(observed), "registered": previous == ""})
}
