package identity

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrandingAcrossPages(t *testing.T) {
	for _, source := range []string{loginPage, "<body><h1>ConnectMe</h1></body>"} {
		var out bytes.Buffer
		if err := brandedTemplate("test", source).Execute(&out, nil); err != nil {
			t.Fatal(err)
		}
		for _, expected := range []string{"ConnectMe by cialinux", "versão " + ReleaseVersion, "https://github.com/cialinux/connectme", "https://cialinux.com"} {
			if !strings.Contains(out.String(), expected) {
				t.Fatalf("missing %q", expected)
			}
		}
	}
	var out bytes.Buffer
	if err := consoleTemplate.Execute(&out, User{DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "versão "+ReleaseVersion) {
		t.Fatal("console footer missing")
	}
}
