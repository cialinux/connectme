// Verify an SSH server's identity, then stop BEFORE user authentication.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: sshverify IP:port known_hosts_file")
	}
	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	_, _, expected, _, _, err := ssh.ParseKnownHosts(data)
	if err != nil {
		panic(err)
	}
	verified := errors.New("identity verified; authentication intentionally skipped")
	cfg := &ssh.ClientConfig{User: "identity-check", Timeout: 5 * time.Second, HostKeyCallback: func(_ string, _ net.Addr, actual ssh.PublicKey) error {
		if !bytes.Equal(actual.Marshal(), expected.Marshal()) {
			return errors.New("HOST KEY MISMATCH")
		}
		return verified
	}}
	conn, err := net.DialTimeout("tcp", os.Args[1], 5*time.Second)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _, _, err = ssh.NewClientConn(conn, os.Args[1], cfg)
	if !errors.Is(err, verified) {
		panic(err)
	}
	fmt.Println("PASS: live SSH identity matches trusted key; no user authentication attempted", ssh.FingerprintSHA256(expected))
}
