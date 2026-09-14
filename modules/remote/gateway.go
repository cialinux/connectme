package remote

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func sessionKey(cookie, id string) [32]byte { return sha256.Sum256([]byte(cookie + ":" + id)) }

func (m *Module) authorize(ctx context.Context, data, name string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://guacamole:8080/guacamole/api/tokens", strings.NewReader(url.Values{"data": {data}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	var auth struct {
		Token string `json:"authToken"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&auth)
	res.Body.Close()
	if err != nil || res.StatusCode != 200 || auth.Token == "" {
		return "", "", errors.New("gateway authorization failed")
	}
	req, _ = http.NewRequestWithContext(ctx, "GET", "http://guacamole:8080/guacamole/api/session/data/json/connections?token="+url.QueryEscape(auth.Token), nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		m.revoke(auth.Token)
		return "", "", err
	}
	var entries map[string]struct {
		Name string `json:"name"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&entries)
	res.Body.Close()
	if err == nil && res.StatusCode == 200 {
		for id, v := range entries {
			if v.Name == name {
				return auth.Token, id, nil
			}
		}
	}
	m.revoke(auth.Token)
	return "", "", errors.New("gateway connection missing")
}
func (m *Module) revoke(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "DELETE", "http://guacamole:8080/guacamole/api/tokens/"+url.PathEscape(token), nil)
	if res, err := http.DefaultClient.Do(req); err == nil {
		res.Body.Close()
	}
}
func (m *Module) closeSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("connectme_session")
	if err != nil {
		http.Error(w, "Sessão necessária", 401)
		return
	}
	key := sessionKey(cookie.Value, r.PathValue("id"))
	m.mu.Lock()
	l, ok := m.leases[key]
	delete(m.leases, key)
	m.mu.Unlock()
	if ok {
		m.revoke(l.Token)
		l.Files.close()
	}
	w.WriteHeader(204)
}

func (m *Module) expireSessions() {
	m.mu.Lock()
	var expired []lease
	for key, l := range m.leases {
		if time.Now().After(l.Expires) {
			expired = append(expired, l)
			delete(m.leases, key)
		}
	}
	m.mu.Unlock()
	for _, l := range expired {
		m.revoke(l.Token)
		l.Files.close()
	}
}
func (m *Module) clientLibrary(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/guacamole/guacamole-common-js/all.min.js"
	r.URL.RawPath = ""
	r.URL.RawQuery = ""
	w.Header().Set("Cache-Control", "no-store")
	m.proxy.ServeHTTP(w, r)
}
