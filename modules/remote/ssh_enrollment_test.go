package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"golang.org/x/crypto/ssh"
	"net"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSSHDiscoveryNeverAuthenticates(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(private)
	var auth atomic.Int32
	cfg := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { auth.Add(1); return nil, nil }}
	cfg.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		conn, e := listener.Accept()
		if e == nil {
			defer conn.Close()
			ssh.NewServerConn(conn, cfg)
		}
	}()
	key, err := probeSSHKey(context.Background(), "127.0.0.1", listener.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatal(err)
	}
	if key != strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) {
		t.Fatal("unexpected key")
	}
	<-finished
	if auth.Load() != 0 {
		t.Fatal("discovery authenticated")
	}
	if sshFingerprint(key) != ssh.FingerprintSHA256(signer.PublicKey()) {
		t.Fatal("fingerprint mismatch")
	}
}
func TestSSHDiscoveryCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := probeSSHKey(ctx, "127.0.0.1", 1); err == nil {
		t.Fatal("cancelled probe succeeded")
	}
	if sshFingerprint("invalid") != "" {
		t.Fatal("invalid key accepted")
	}
}
