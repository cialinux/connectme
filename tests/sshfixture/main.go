// Disposable SSH protocol fixture. Never executes an operating-system shell.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
)

func main() {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(key)
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if c.User() == "fixture" && string(p) == "Fixture-only-2026!" {
			return nil, nil
		}
		return nil, fmt.Errorf("denied")
	}}
	cfg.AddHostKey(signer)
	// Multiple algorithms detect discovery/gateway preference mismatches.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	rsaSigner, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		panic(err)
	}
	cfg.AddHostKey(rsaSigner)
	fmt.Print(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	listener, err := net.Listen("tcp", ":2222")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go func() {
			defer conn.Close()
			_, channels, reqs, err := ssh.NewServerConn(conn, cfg)
			if err != nil {
				return
			}
			go ssh.DiscardRequests(reqs)
			for pending := range channels {
				if pending.ChannelType() != "session" {
					pending.Reject(ssh.UnknownChannelType, "session only")
					continue
				}
				ch, requests, _ := pending.Accept()
				go func() {
					defer ch.Close()
					for r := range requests {
						switch r.Type {
						case "pty-req", "window-change":
							r.Reply(true, nil)
						case "shell":
							r.Reply(true, nil)
							io.WriteString(ch, "\x1b[2J\x1b[HConnectMe SSH fixture ready\r\n> ")
							buf := make([]byte, 1024)
							for {
								n, e := ch.Read(buf)
								if e != nil {
									return
								}
								fmt.Printf("FIXTURE_ECHO_HEX:%x\n", buf[:n])
								ch.Write(buf[:n])
							}
						default:
							r.Reply(false, nil)
						}
					}
				}()
			}
		}()
	}
}
