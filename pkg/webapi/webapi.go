package webapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), "application/json") {
		Problem(w, 415, "content_type", "use application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		Problem(w, 400, "invalid_json", "JSON inválido")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		Problem(w, 400, "invalid_json", "esperado um único objeto JSON")
		return false
	}
	return true
}
func Respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func Problem(w http.ResponseWriter, status int, code, detail string) {
	Respond(w, status, map[string]any{"error": map[string]string{"code": code, "detail": detail}})
}
func Page(r *http.Request) (limit, offset int) {
	limit = 50
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 200 {
		limit = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n >= 0 {
		offset = n
	}
	return
}
