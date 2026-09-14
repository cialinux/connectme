package webapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectTrailingJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ok"}{"name":"ignored"}`))
	r.Header.Set("Content-Type", "application/json")
	var input struct{ Name string }
	w := httptest.NewRecorder()
	if Decode(w, r, &input) || w.Code != 400 {
		t.Fatal("trailing payload accepted")
	}
}
