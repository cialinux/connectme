package remote

import (
	"errors"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"strconv"
	"strings"
)

// Trust is provisioned independently of the network handshake. Unknown or
// changed host keys are never silently accepted. Only exact IP:port entries.
func sshHostKey(address string, port int) (string, error) {
	path := os.Getenv("CONNECTME_SSH_KNOWN_HOSTS")
	if path == "" {
		return "", errors.New("configure a chave pública confiável do servidor SSH")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 1<<20 {
		return "", errors.New("arquivo de identidade SSH indisponível ou excessivo")
	}
	return findSSHHostKey(data, address, port)
}

func findSSHHostKey(data []byte, address string, port int) (string, error) {
	target := net.JoinHostPort(address, strconv.Itoa(port))
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		marker, names, key, _, _, err := ssh.ParseKnownHosts([]byte(line))
		if err != nil || marker != "" {
			return "", errors.New("entrada inválida no arquivo de identidade SSH")
		}
		for _, name := range names {
			if name == "["+address+"]:"+strconv.Itoa(port) || name == target || (port == 22 && name == address) {
				return "[" + address + "]:" + strconv.Itoa(port) + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))), nil
			}
		}
	}
	return "", errors.New("servidor SSH sem chave pública aprovada; configure ssh_known_hosts")
}
