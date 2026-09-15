package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/connectme/connectme/core/config"
	"github.com/connectme/connectme/core/modules"
	"html/template"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const cookieName = "connectme_session"
const csrfCookieName = "connectme_csrf"

type contextKey string

const userKey contextKey = "identity.user"

func CurrentUser(ctx context.Context) (User, bool) { u, ok := ctx.Value(userKey).(User); return u, ok }

type Authorizer interface {
	Allowed(context.Context, string, string) (bool, error)
}

type Module struct {
	service    *Service
	security   config.Security
	page       *template.Template
	rateMu     sync.Mutex
	attempts   map[string][]time.Time
	authorizer Authorizer
}

func New(s *Service, c config.Security, a Authorizer) *Module {
	return &Module{service: s, security: c, page: brandedTemplate("login", loginPage), attempts: map[string][]time.Time{}, authorizer: a}
}
func (m *Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: "identity", Version: "0.1.0", Dependencies: []string{"audit"}, Critical: true}
}
func (m *Module) Register(r modules.Registrar) error {
	r.Handle(http.MethodGet, "/", http.HandlerFunc(m.index))
	r.Handle(http.MethodPost, "/api/v1/auth/login", http.HandlerFunc(m.login))
	r.Handle(http.MethodPost, "/api/v1/auth/logout", m.require(http.HandlerFunc(m.logout)))
	r.Handle(http.MethodGet, "/api/v1/auth/me", m.require(http.HandlerFunc(m.me)))
	r.Handle(http.MethodPost, "/api/v1/auth/mfa/setup", m.require(http.HandlerFunc(m.mfaSetup)))
	r.Handle(http.MethodPost, "/api/v1/auth/mfa/confirm", m.require(http.HandlerFunc(m.mfaConfirm)))
	r.Handle(http.MethodPost, "/api/v1/auth/password", m.require(http.HandlerFunc(m.changePassword)))
	r.Handle(http.MethodGet, "/app", m.require(http.HandlerFunc(m.console)))
	r.Handle(http.MethodGet, "/app/assets/console.js", m.require(http.HandlerFunc(m.consoleScript)))
	r.Handle(http.MethodGet, "/api/v1/users", m.Protect("users.read", http.HandlerFunc(m.listUsers)))
	r.Handle(http.MethodPost, "/api/v1/users", m.Protect("users.manage", http.HandlerFunc(m.createUser)))
	r.Handle(http.MethodPut, "/api/v1/users/{id}", m.Protect("users.manage", http.HandlerFunc(m.editUser)))
	r.Handle(http.MethodDelete, "/api/v1/users/{id}", m.Protect("users.manage", http.HandlerFunc(m.editUser)))
	return nil
}
func (m *Module) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := m.service.ListUsers(r.Context())
	if err != nil {
		problem(w, 500, "users_failed", "não foi possível consultar utilizadores")
		return
	}
	respond(w, 200, map[string]any{"items": users})
}
func (m *Module) createUser(w http.ResponseWriter, r *http.Request) {
	if !jsonContent(r) {
		problem(w, 415, "content_type", "use application/json")
		return
	}
	var in struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		return
	}
	u, err := m.service.CreateAdmin(r.Context(), in.Email, in.DisplayName, in.Password)
	if err != nil {
		problem(w, 400, "user_invalid", err.Error())
		return
	}
	respond(w, 201, u)
}
func (m *Module) Protect(permission string, next http.Handler) http.Handler {
	return m.require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value(userKey).(User)
		if u.MustChangePassword && r.URL.Path != "/api/v1/auth/password" {
			problem(w, 403, "password_change_required", "altere a senha inicial")
			return
		}
		ok, err := m.authorizer.Allowed(r.Context(), u.ID, permission)
		if err != nil || !ok {
			problem(w, 403, "forbidden", "permissão insuficiente")
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		status := &mutationWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(status, r)
		outcome := "success"
		if status.status >= 400 {
			outcome = "failure"
		}
		m.service.record(r.Context(), AuditEvent{ActorID: u.ID, Action: "admin." + strings.ToLower(r.Method), Outcome: outcome, ResourceID: r.PathValue("id"), ResourceType: permission, CorrelationID: w.Header().Get("X-Correlation-ID"), Metadata: map[string]any{"path": r.URL.Path, "status": status.status}})
	}))
}

type mutationWriter struct {
	http.ResponseWriter
	status int
}

func (w *mutationWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *mutationWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (m *Module) Allowed(ctx context.Context, userID, permission string) (bool, error) {
	return m.authorizer.Allowed(ctx, userID, permission)
}
func (m *Module) dashboard(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboardPage.Execute(w, u)
}
func (m *Module) changePassword(w http.ResponseWriter, r *http.Request) {
	if !jsonContent(r) {
		problem(w, 415, "content_type", "use application/json")
		return
	}
	var in struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if err := decode(w, r, &in); err != nil {
		return
	}
	if err := m.service.ChangePassword(r.Context(), r.Context().Value(userKey).(User), in.Current, in.Next); err != nil {
		problem(w, 400, "password_change_failed", err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.security.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	respond(w, 200, map[string]string{"status": "password_changed", "next": "login_again"})
}
func (m *Module) Start(context.Context) error { return nil }
func (m *Module) Stop(context.Context) error  { return nil }
func (m *Module) Health(context.Context) modules.HealthStatus {
	return modules.HealthStatus{Status: "ok", CheckedAt: time.Now().UTC()}
}
func (m *Module) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = m.page.Execute(w, nil)
}
func (m *Module) login(w http.ResponseWriter, r *http.Request) {
	if !jsonContent(r) {
		problem(w, 415, "content_type", "use application/json")
		return
	}
	var in struct{ Email, Password, TOTP string }
	if err := decode(w, r, &in); err != nil {
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !m.allowLogin(ip) {
		w.Header().Set("Retry-After", "60")
		problem(w, 429, "rate_limited", "muitas tentativas; tente novamente mais tarde")
		return
	}
	u, token, err := m.service.Login(r.Context(), in.Email, in.Password, in.TOTP, ip, r.UserAgent())
	if err != nil {
		code := "invalid_credentials"
		if errors.Is(err, ErrMFARequired) {
			code = "mfa_required"
		}
		problem(w, 401, code, "autenticação falhou")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: m.security.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: int(m.security.SessionTTL.Seconds())})
	csrfBytes := make([]byte, 32)
	if _, err = rand.Read(csrfBytes); err != nil {
		problem(w, 500, "csrf_failed", "falha ao criar sessão")
		return
	}
	csrf := hex.EncodeToString(csrfBytes)
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: csrf, Path: "/", HttpOnly: false, Secure: m.security.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: int(m.security.SessionTTL.Seconds())})
	respond(w, 200, u)
}
func (m *Module) allowLogin(ip string) bool {
	m.rateMu.Lock()
	defer m.rateMu.Unlock()
	cut := time.Now().Add(-time.Minute)
	kept := m.attempts[ip][:0]
	for _, at := range m.attempts[ip] {
		if at.After(cut) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= 10 {
		m.attempts[ip] = kept
		return false
	}
	m.attempts[ip] = append(kept, time.Now())
	return true
}
func (m *Module) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(cookieName)
	if c != nil {
		_ = m.service.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.security.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: "", Path: "/", Secure: m.security.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (m *Module) me(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, r.Context().Value(userKey))
}
func (m *Module) mfaSetup(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	secret, uri, err := m.service.BeginMFA(r.Context(), u)
	if err != nil {
		problem(w, 500, "mfa_setup_failed", "não foi possível configurar MFA")
		return
	}
	respond(w, 200, map[string]string{"secret": secret, "otpauth_uri": uri})
}
func (m *Module) mfaConfirm(w http.ResponseWriter, r *http.Request) {
	if !jsonContent(r) {
		problem(w, 415, "content_type", "use application/json")
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := decode(w, r, &in); err != nil {
		return
	}
	codes, err := m.service.ConfirmMFA(r.Context(), r.Context().Value(userKey).(User), in.Code)
	if err != nil {
		problem(w, 400, "invalid_totp", "código inválido")
		return
	}
	respond(w, 200, map[string]any{"recovery_codes": codes, "warning": "guarde estes códigos agora"})
}
func (m *Module) require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			problem(w, 401, "unauthorized", "autenticação necessária")
			return
		}
		u, err := m.service.Authenticate(r.Context(), c.Value)
		if err != nil {
			problem(w, 401, "unauthorized", "sessão inválida ou expirada")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if u.MustChangePassword && r.URL.Path != "/app/assets/console.js" && r.URL.Path != "/app" && r.URL.Path != "/api/v1/auth/me" && r.URL.Path != "/api/v1/auth/logout" && r.URL.Path != "/api/v1/auth/password" {
			problem(w, 403, "password_change_required", "altere a senha inicial")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			csrfCookie, cookieErr := r.Cookie(csrfCookieName)
			header := r.Header.Get("X-CSRF-Token")
			if cookieErr != nil || len(header) != 64 || len(header) != len(csrfCookie.Value) || subtle.ConstantTimeCompare([]byte(header), []byte(csrfCookie.Value)) != 1 {
				problem(w, 403, "csrf_invalid", "token CSRF inválido")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
func jsonContent(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), "application/json")
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		problem(w, 400, "invalid_json", "JSON inválido")
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		problem(w, 400, "invalid_json", "esperado um único objeto JSON")
		return errors.New("trailing JSON")
	}
	return nil
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, detail string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "detail": detail}})
}

const loginPage = `<!doctype html><html lang="pt"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>ConnectMe</title><style>body{font:16px system-ui;max-width:32rem;margin:10vh auto;padding:1rem;background:#111827;color:#f9fafb}form{display:grid;gap:1rem}input,button{font:inherit;padding:.8rem;border-radius:.5rem}button{background:#2563eb;color:white;border:0}</style></head><body><h1>ConnectMe</h1><p>Central privada de conexões</p><form id="login"><input name="email" placeholder="Utilizador" required><input name="password" type="password" placeholder="Senha" required><input name="totp" inputmode="numeric" placeholder="MFA (se habilitado)"><button>Entrar</button></form><pre id="result"></pre><script>login.onsubmit=async e=>{e.preventDefault();let body=Object.fromEntries(new FormData(login));let r=await fetch('/api/v1/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});if(r.ok)location='/app';else result.textContent=JSON.stringify(await r.json())}</script></body></html>`

var dashboardPage = brandedTemplate("dashboard", `<!doctype html><html lang="pt"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>ConnectMe</title><style>body{font:16px system-ui;max-width:60rem;margin:3rem auto;padding:1rem;background:#111827;color:#f9fafb}.card{background:#1f2937;padding:1.2rem;border-radius:.7rem;margin:1rem 0}input,button{font:inherit;padding:.7rem;margin:.3rem}button{background:#2563eb;color:#fff;border:0;border-radius:.4rem}</style></head><body><h1>ConnectMe</h1><p>Olá, {{.DisplayName}}</p>{{if .MustChangePassword}}<section class="card"><h2>Troca obrigatória de senha</h2><p>A credencial inicial admin/admin é temporária.</p><form id="pw"><input name="current_password" type="password" placeholder="Senha atual" required><input name="new_password" type="password" placeholder="Nova senha (mín. 12)" required><button>Alterar senha</button></form><pre id="status"></pre></section>{{else}}<section class="card"><h2>Core operacional</h2><p>Identity, RBAC, locations, hosts, credentials e connections estão ativos.</p></section>{{end}}<script>function cookie(n){return document.cookie.split('; ').find(x=>x.startsWith(n+'='))?.split('=')[1]||''}if(window.pw)pw.onsubmit=async e=>{e.preventDefault();let r=await fetch('/api/v1/auth/password',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':cookie('connectme_csrf')},body:JSON.stringify(Object.fromEntries(new FormData(pw)))});status.textContent=JSON.stringify(await r.json());if(r.ok)setTimeout(()=>location='/',800)}</script></body></html>`)
