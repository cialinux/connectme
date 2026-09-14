package identity

import (
	"testing"
	"time"
)

func TestPasswordHash(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(encoded, "correct horse battery staple") {
		t.Fatal("senha correta rejeitada")
	}
	if VerifyPassword(encoded, "wrong password") {
		t.Fatal("senha incorreta aceita")
	}
}
func TestPasswordMinimum(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("senha curta aceita")
	}
}
func TestTOTPKnownSecret(t *testing.T) {
	if !VerifyTOTP("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", time.Unix(59, 0)) {
		t.Fatal("vetor TOTP rejeitado")
	}
}
