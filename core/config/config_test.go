package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	setKeys(t)
	t.Setenv("CONNECTME_DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("esperava erro sem database URL")
	}
}

func TestCriticalModuleCannotBeDisabled(t *testing.T) {
	setKeys(t)
	t.Setenv("CONNECTME_DATABASE_URL", "postgres://example")
	t.Setenv("CONNECTME_MODULES", "system=false")
	if _, err := Load(); err == nil {
		t.Fatal("esperava rejeição do módulo crítico")
	}
}

func setKeys(t *testing.T) {
	t.Helper()
	t.Setenv("CONNECTME_MASTER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("CONNECTME_AUDIT_HMAC_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
}
