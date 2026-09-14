package connections

import (
	"context"
	"testing"
)

type missing struct{}

func (missing) Exists(context.Context, string) (bool, error) { return false, nil }
func TestConnectionRejectsMissingHostBeforeWriting(t *testing.T) {
	s := NewService(nil, missing{}, missing{})
	_, err := s.Create(context.Background(), Connection{Name: "SSH", Protocol: "ssh", Port: 22, HostID: "missing", CredentialRefID: "missing"}, "actor")
	if err == nil || err.Error() != "host inexistente ou desativado" {
		t.Fatalf("unexpected: %v", err)
	}
}
