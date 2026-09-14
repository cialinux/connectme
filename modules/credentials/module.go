package credentials

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"github.com/connectme/connectme/core/encryption"
	"github.com/connectme/connectme/core/modules"
	"github.com/connectme/connectme/modules/identity"
	"github.com/connectme/connectme/pkg/webapi"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"strings"
	"time"
)

type Secret struct {
	Username, Domain    string
	Name, Type, OwnerID string
	Value               []byte
}
type SecretRef struct{ ID string }
type Metadata struct {
	Username  string     `json:"username"`
	Domain    string     `json:"domain"`
	Enabled   bool       `json:"enabled"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	OwnerID   string     `json:"owner_id"`
	CreatedAt time.Time  `json:"created_at"`
	RotatedAt *time.Time `json:"rotated_at,omitempty"`
}
type SecretProvider interface {
	Store(context.Context, Secret) (SecretRef, error)
	Retrieve(context.Context, SecretRef) (Secret, error)
	Rotate(context.Context, SecretRef, Secret) error
	Delete(context.Context, SecretRef) error
}
type LocalProvider struct {
	pool   *pgxpool.Pool
	master *encryption.Cipher
}

func NewProvider(p *pgxpool.Pool, m *encryption.Cipher) *LocalProvider { return &LocalProvider{p, m} }
func (p *LocalProvider) Exists(ctx context.Context, id string) (bool, error) {
	var found bool
	err := p.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM credentials.secret_refs WHERE id=$1 AND deleted_at IS NULL AND enabled)", id).Scan(&found)
	return found, err
}
func (p *LocalProvider) Store(ctx context.Context, s Secret) (SecretRef, error) {
	if len(s.Value) == 0 {
		return SecretRef{}, errors.New("secret vazio")
	}
	id := newUUID()
	dataKey := make([]byte, 32)
	if _, err := rand.Read(dataKey); err != nil {
		return SecretRef{}, err
	}
	wrapped, err := p.master.Encrypt(dataKey, []byte("dek:"+id+":1"))
	if err != nil {
		return SecretRef{}, err
	}
	value, err := seal(dataKey, s.Value, []byte("secret:"+id+":"+s.Type))
	for i := range dataKey {
		dataKey[i] = 0
	}
	if err != nil {
		return SecretRef{}, err
	}
	_, err = p.pool.Exec(ctx, "INSERT INTO credentials.secret_refs(id,name,type,encrypted_data_key,ciphertext,owner_id,username,domain) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", id, s.Name, s.Type, wrapped, value, s.OwnerID, s.Username, s.Domain)
	return SecretRef{id}, err
}
func (p *LocalProvider) List(ctx context.Context, limit, offset int) ([]Metadata, error) {
	rows, err := p.pool.Query(ctx, "SELECT id,name,type,owner_id,created_at,rotated_at,username,domain,enabled FROM credentials.secret_refs WHERE deleted_at IS NULL ORDER BY lower(name) LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Metadata{}
	for rows.Next() {
		var v Metadata
		if err = rows.Scan(&v.ID, &v.Name, &v.Type, &v.OwnerID, &v.CreatedAt, &v.RotatedAt, &v.Username, &v.Domain, &v.Enabled); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *LocalProvider) Retrieve(ctx context.Context, r SecretRef) (Secret, error) {
	var s Secret
	var wrapped, value []byte
	var version int
	err := p.pool.QueryRow(ctx, "SELECT name,type,owner_id,encrypted_data_key,ciphertext,key_version FROM credentials.secret_refs WHERE id=$1 AND deleted_at IS NULL", r.ID).Scan(&s.Name, &s.Type, &s.OwnerID, &wrapped, &value, &version)
	if err != nil {
		return s, err
	}
	key, err := p.master.Decrypt(wrapped, []byte("dek:"+r.ID+":"+itoa(version)))
	if err != nil {
		return s, err
	}
	s.Value, err = open(key, value, []byte("secret:"+r.ID+":"+s.Type))
	for i := range key {
		key[i] = 0
	}
	return s, err
}
func (p *LocalProvider) Rotate(ctx context.Context, r SecretRef, s Secret) error {
	old, err := p.Retrieve(ctx, r)
	if err != nil {
		return err
	}
	if s.Type == "" {
		s.Type = old.Type
	}
	dataKey := make([]byte, 32)
	_, err = rand.Read(dataKey)
	if err != nil {
		return err
	}
	wrapped, err := p.master.Encrypt(dataKey, []byte("dek:"+r.ID+":2"))
	if err != nil {
		return err
	}
	value, err := seal(dataKey, s.Value, []byte("secret:"+r.ID+":"+s.Type))
	for i := range dataKey {
		dataKey[i] = 0
	}
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, "UPDATE credentials.secret_refs SET name=$2,type=$3,encrypted_data_key=$4,ciphertext=$5,key_version=2,rotated_at=now() WHERE id=$1 AND deleted_at IS NULL", r.ID, s.Name, s.Type, wrapped, value)
	return err
}
func (p *LocalProvider) Delete(ctx context.Context, r SecretRef) error {
	_, err := p.pool.Exec(ctx, "UPDATE credentials.secret_refs SET deleted_at=now(),ciphertext='\\x' WHERE id=$1 AND deleted_at IS NULL", r.ID)
	return err
}
func seal(key, plain, aad []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	a, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}
	n := make([]byte, a.NonceSize())
	if _, err = io.ReadFull(rand.Reader, n); err != nil {
		return nil, err
	}
	return append(n, a.Seal(nil, n, plain, aad)...), nil
}
func open(key, value, aad []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	a, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}
	if len(value) < a.NonceSize() {
		return nil, errors.New("ciphertext inválido")
	}
	return a.Open(nil, value[:a.NonceSize()], value[a.NonceSize():], aad)
}

type Protector interface {
	Protect(string, http.Handler) http.Handler
}
type Module struct {
	provider *LocalProvider
	p        Protector
}

func New(provider *LocalProvider, p Protector) *Module { return &Module{provider, p} }
func (*Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "credentials", Version: "0.2.0", Dependencies: []string{"authorization", "audit"}}
}
func (m *Module) Register(r modules.Registrar) error {
	m.editRoutes(r)
	r.Handle("GET", "/api/v1/credentials", m.p.Protect("credentials.read", http.HandlerFunc(m.list)))
	r.Handle("POST", "/api/v1/credentials", m.p.Protect("credentials.manage", http.HandlerFunc(m.create)))
	return nil
}
func (m *Module) list(w http.ResponseWriter, r *http.Request) {
	l, o := webapi.Page(r)
	items, err := m.provider.List(r.Context(), l, o)
	if err != nil {
		webapi.Problem(w, 500, "credentials_failed", "falha ao consultar credenciais")
		return
	}
	webapi.Respond(w, 200, map[string]any{"items": items, "limit": l, "offset": o})
}
func (m *Module) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Domain   string `json:"domain"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Value    string `json:"value"`
	}
	if !webapi.Decode(w, r, &in) {
		return
	}
	in.Type = strings.ToLower(in.Type)
	if in.Type != "password" && in.Type != "ssh-key" && in.Type != "windows" {
		webapi.Problem(w, 400, "credential_invalid", "tipo inválido")
		return
	}
	u, _ := identity.CurrentUser(r.Context())
	ref, err := m.provider.Store(r.Context(), Secret{Name: strings.TrimSpace(in.Name), Type: in.Type, OwnerID: u.ID, Value: []byte(in.Value), Username: in.Username, Domain: in.Domain})
	in.Value = ""
	if err != nil {
		webapi.Problem(w, 400, "credential_invalid", err.Error())
		return
	}
	webapi.Respond(w, 201, map[string]string{"id": ref.ID, "name": in.Name, "type": in.Type})
}
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
func (*Module) Health(context.Context) modules.HealthStatus {
	return modules.HealthStatus{Status: "ok", CheckedAt: time.Now().UTC()}
}
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	const h = "0123456789abcdef"
	out := make([]byte, 36)
	j := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[j] = '-'
			j++
		}
		out[j] = h[v>>4]
		out[j+1] = h[v&15]
		j += 2
	}
	return string(out)
}
func itoa(v int) string {
	if v == 1 {
		return "1"
	}
	return "2"
}
