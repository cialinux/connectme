package identity

import (
	"bytes"
	"strings"
	"testing"
)

func TestConsoleEscapesUserAndRestrictsBootstrap(t *testing.T) {
	for _, restricted := range []bool{true, false} {
		var out bytes.Buffer
		if err := consoleTemplate.Execute(&out, User{DisplayName: "<script>alert(1)</script>", MustChangePassword: restricted}); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "<script>alert(1)</script>") {
			t.Fatal("unescaped user name")
		}
		if strings.Contains(out.String(), `id="editor"`) == restricted {
			t.Fatal("bootstrap form restriction incorrect")
		}
	}
}
func TestMalformedArgonParameters(t *testing.T) {
	for _, parameters := range []string{"m=65536,t=0,p=2", "m=65536,t=3,p=0", "m=0,t=3,p=2"} {
		encoded := "$argon2id$v=19$" + parameters + "$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"
		if VerifyPassword(encoded, "anything") {
			t.Fatal("invalid parameters accepted")
		}
	}
}
