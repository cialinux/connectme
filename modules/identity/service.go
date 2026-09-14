package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/connectme/connectme/core/config"
	"github.com/connectme/connectme/core/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

var ErrInvalidCredentials = errors.New("credenciais inválidas")
var ErrMFARequired = errors.New("MFA obrigatório")
var ErrUnauthorized = errors.New("não autenticado")

type Auditor interface {
	Record(context.Context, AuditEvent) error
}
type AuditorFunc func(context.Context, AuditEvent) error

func (f AuditorFunc) Record(ctx context.Context, e AuditEvent) error { return f(ctx, e) }

type AuditEvent struct {
	ActorID, Action, Outcome, ResourceType, ResourceID, CorrelationID, SourceIP string
	Metadata                                                                    map[string]any
}
type Service struct {
	pool   *pgxpool.Pool
	cipher *encryption.Cipher
	audit  Auditor
	cfg    config.Security
	now    func() time.Time
}
type User struct {
	Enabled            bool   `json:"enabled"`
	ID                 string `json:"id"`
	Email              string `json:"email"`
	DisplayName        string `json:"display_name"`
	MFAEnabled         bool   `json:"mfa_enabled"`
	MustChangePassword bool   `json:"must_change_password"`
}

func NewService(p *pgxpool.Pool, c *encryption.Cipher, a Auditor, cfg config.Security) *Service {
	return &Service{p, c, a, cfg, time.Now}
}

// Audit records metadata only; callers must never pass clipboard or file contents.
func (s *Service) Audit(ctx context.Context, event AuditEvent) error {
	return s.audit.Record(ctx, event)
}
func (s *Service) EnsureDefaultAdmin(ctx context.Context) (User, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(0x434d41444d494e)); err != nil {
		return User{}, false, err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM identity.users WHERE deleted_at IS NULL").Scan(&count); err != nil {
		return User{}, false, err
	}
	if count > 0 {
		return User{}, false, nil
	}
	hash, err := hashPassword("admin")
	if err != nil {
		return User{}, false, err
	}
	u := User{ID: newUUID(), Email: "admin", DisplayName: "Administrator", MustChangePassword: true}
	if _, err = tx.Exec(ctx, "INSERT INTO identity.users(id,email,display_name,must_change_password) VALUES($1,$2,$3,true)", u.ID, u.Email, u.DisplayName); err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO identity.password_credentials(user_id,password_hash) VALUES($1,$2)", u.ID, hash)
	}
	if err != nil {
		return User{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, false, err
	}
	s.record(ctx, AuditEvent{ActorID: u.ID, Action: "identity.default_admin.created", Outcome: "success", ResourceType: "user", ResourceID: u.ID})
	return u, true, nil
}
func (s *Service) CreateAdmin(ctx context.Context, email, name, password string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return User{}, errors.New("email inválido")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	id := newUUID()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "INSERT INTO identity.users(id,email,display_name) VALUES($1,$2,$3)", id, email, name); err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO identity.password_credentials(user_id,password_hash) VALUES($1,$2)", id, hash)
	}
	if err != nil {
		return User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	s.record(ctx, AuditEvent{ActorID: id, Action: "identity.admin.created", Outcome: "success", ResourceType: "user", ResourceID: id})
	return User{ID: id, Email: email, DisplayName: name}, nil
}
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT u.id,u.email,u.display_name,EXISTS(SELECT 1 FROM identity.totp_factors t WHERE t.user_id=u.id AND NOT t.pending),u.must_change_password,u.enabled FROM identity.users u WHERE u.deleted_at IS NULL ORDER BY lower(u.email)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.MFAEnabled, &u.MustChangePassword, &u.Enabled); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Service) Login(ctx context.Context, email, password, mfa, ip, ua string) (User, string, error) {
	var u User
	var hash string
	var enabled bool
	var locked *time.Time
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.email,u.display_name,u.enabled,u.locked_until,p.password_hash,EXISTS(SELECT 1 FROM identity.totp_factors t WHERE t.user_id=u.id AND NOT t.pending),u.must_change_password FROM identity.users u JOIN identity.password_credentials p ON p.user_id=u.id WHERE lower(u.email)=lower($1) AND u.deleted_at IS NULL`, strings.TrimSpace(email)).Scan(&u.ID, &u.Email, &u.DisplayName, &enabled, &locked, &hash, &u.MFAEnabled, &u.MustChangePassword)
	if err != nil || !enabled || (locked != nil && locked.After(s.now())) || !VerifyPassword(hash, password) {
		if err == nil {
			_, _ = s.pool.Exec(ctx, `UPDATE identity.users SET failed_attempts=failed_attempts+1,locked_until=CASE WHEN failed_attempts+1>=5 THEN now()+interval '15 minutes' ELSE locked_until END WHERE id=$1`, u.ID)
		}
		s.record(ctx, AuditEvent{ActorID: u.ID, Action: "identity.login", Outcome: "failure", ResourceType: "user", ResourceID: u.ID, SourceIP: ip})
		return User{}, "", ErrInvalidCredentials
	}
	if u.MFAEnabled {
		if ok, err := s.verifySecondFactor(ctx, u.ID, mfa); err != nil || !ok {
			s.record(ctx, AuditEvent{ActorID: u.ID, Action: "identity.login", Outcome: "mfa_failure", ResourceType: "user", ResourceID: u.ID, SourceIP: ip})
			return User{}, "", ErrMFARequired
		}
	}
	_, _ = s.pool.Exec(ctx, "UPDATE identity.users SET failed_attempts=0,locked_until=NULL WHERE id=$1", u.ID)
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return User{}, "", err
	}
	token := hex.EncodeToString(tokenBytes)
	sum := sha256.Sum256([]byte(token))
	_, err = s.pool.Exec(ctx, `INSERT INTO identity.sessions(id,user_id,token_hash,expires_at,ip,user_agent) VALUES($1,$2,$3,$4,NULLIF($5,'')::inet,$6)`, newUUID(), u.ID, sum[:], s.now().Add(s.cfg.SessionTTL), ip, ua)
	if err != nil {
		return User{}, "", err
	}
	s.record(ctx, AuditEvent{ActorID: u.ID, Action: "identity.login", Outcome: "success", ResourceType: "user", ResourceID: u.ID, SourceIP: ip})
	return u, token, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	sum := sha256.Sum256([]byte(token))
	var u User
	var last, expires time.Time
	err := s.pool.QueryRow(ctx, `SELECT u.id,u.email,u.display_name,s.last_seen_at,s.expires_at,EXISTS(SELECT 1 FROM identity.totp_factors t WHERE t.user_id=u.id AND NOT t.pending),u.must_change_password FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.enabled AND u.deleted_at IS NULL`, sum[:]).Scan(&u.ID, &u.Email, &u.DisplayName, &last, &expires, &u.MFAEnabled, &u.MustChangePassword)
	if err != nil || s.now().Sub(last) > s.cfg.IdleTTL {
		return User{}, ErrUnauthorized
	}
	if s.now().Sub(last) > time.Minute {
		_, _ = s.pool.Exec(ctx, "UPDATE identity.sessions SET last_seen_at=now() WHERE token_hash=$1", sum[:])
	}
	return u, nil
}
func (s *Service) Logout(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	_, err := s.pool.Exec(ctx, "UPDATE identity.sessions SET revoked_at=now() WHERE token_hash=$1", sum[:])
	return err
}
func (s *Service) ChangePassword(ctx context.Context, u User, current, next string) error {
	var encoded string
	if err := s.pool.QueryRow(ctx, "SELECT password_hash FROM identity.password_credentials WHERE user_id=$1", u.ID).Scan(&encoded); err != nil {
		return err
	}
	if !VerifyPassword(encoded, current) {
		return ErrInvalidCredentials
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "UPDATE identity.password_credentials SET password_hash=$2,changed_at=now() WHERE user_id=$1", u.ID, hash); err == nil {
		_, err = tx.Exec(ctx, "UPDATE identity.users SET must_change_password=false,updated_at=now() WHERE id=$1", u.ID)
	}
	if err == nil {
		_, err = tx.Exec(ctx, "UPDATE identity.sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL", u.ID)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err == nil {
		s.record(ctx, AuditEvent{ActorID: u.ID, Action: "identity.password.changed", Outcome: "success", ResourceType: "user", ResourceID: u.ID})
	}
	return err
}
func (s *Service) record(ctx context.Context, e AuditEvent) {
	if s.audit != nil {
		_ = s.audit.Record(ctx, e)
	}
}
func (s *Service) BeginMFA(ctx context.Context, u User) (string, string, error) {
	secret, uri, err := NewTOTP(u.Email)
	if err != nil {
		return "", "", err
	}
	enc, err := s.cipher.Encrypt([]byte(secret), []byte("totp:"+u.ID))
	if err != nil {
		return "", "", err
	}
	result, err := s.pool.Exec(ctx, `INSERT INTO identity.totp_factors(user_id,encrypted_secret,pending) VALUES($1,$2,true) ON CONFLICT(user_id) DO UPDATE SET encrypted_secret=excluded.encrypted_secret,pending=true,confirmed_at=NULL WHERE identity.totp_factors.pending`, u.ID, enc)
	if err == nil && result.RowsAffected() == 0 {
		return "", "", errors.New("MFA já está ativo")
	}
	return secret, uri, err
}
func (s *Service) ConfirmMFA(ctx context.Context, u User, code string) ([]string, error) {
	var enc []byte
	if err := s.pool.QueryRow(ctx, "SELECT encrypted_secret FROM identity.totp_factors WHERE user_id=$1 AND pending", u.ID).Scan(&enc); err != nil {
		return nil, err
	}
	plain, err := s.cipher.Decrypt(enc, []byte("totp:"+u.ID))
	if err != nil || !VerifyTOTP(string(plain), code, s.now()) {
		return nil, errors.New("código TOTP inválido")
	}
	codes := make([]string, 10)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM identity.recovery_codes WHERE user_id=$1", u.ID); err != nil {
		return nil, err
	}
	for i := range codes {
		codes[i] = recoveryCode()
		sum := sha256.Sum256([]byte(normalizeCode(codes[i])))
		if _, err = tx.Exec(ctx, "INSERT INTO identity.recovery_codes(user_id,code_hash) VALUES($1,$2)", u.ID, sum[:]); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, "UPDATE identity.totp_factors SET pending=false,confirmed_at=now() WHERE user_id=$1", u.ID); err != nil {
		return nil, err
	}
	return codes, tx.Commit(ctx)
}
func (s *Service) verifySecondFactor(ctx context.Context, userID, code string) (bool, error) {
	var enc []byte
	if err := s.pool.QueryRow(ctx, "SELECT encrypted_secret FROM identity.totp_factors WHERE user_id=$1 AND NOT pending", userID).Scan(&enc); err != nil {
		return false, err
	}
	plain, err := s.cipher.Decrypt(enc, []byte("totp:"+userID))
	if err == nil && VerifyTOTP(string(plain), code, s.now()) {
		return true, nil
	}
	sum := sha256.Sum256([]byte(normalizeCode(code)))
	tag, err := s.pool.Exec(ctx, "UPDATE identity.recovery_codes SET used_at=now() WHERE user_id=$1 AND code_hash=$2 AND used_at IS NULL", userID, sum[:])
	return err == nil && tag.RowsAffected() == 1, err
}
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
