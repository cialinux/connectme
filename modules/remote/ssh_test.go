package remote

import "testing"

func TestSSHHostIdentity(t *testing.T) {
	const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIkz+/AgDpro59s8eaaZoBjj9xBYD7s706EQiBvwivwt"
	data := []byte("# trusted\n[192.168.1.248]:22 " + key + "\n")
	if _, err := findSSHHostKey(data, "192.168.1.248", 22); err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct {
		ip   string
		port int
	}{{"192.168.1.249", 22}, {"192.168.1.248", 2222}} {
		if _, err := findSSHHostKey(data, v.ip, v.port); err == nil {
			t.Fatal("trust escaped exact endpoint")
		}
	}
	for _, bad := range []string{"", "malformed", "@revoked 192.168.1.248 " + key} {
		if _, err := findSSHHostKey([]byte(bad), "192.168.1.248", 22); err == nil {
			t.Fatal("invalid trust accepted")
		}
	}
}
