package identity

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed console.html console.js workspace.js transfers.js clipboard.js
var consoleFiles embed.FS
var consoleTemplate = func() *template.Template {
	source, err := consoleFiles.ReadFile("console.html")
	if err != nil {
		panic(err)
	}
	return brandedTemplate("console", string(source))
}()

func (m *Module) consoleScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	b, _ := consoleFiles.ReadFile("console.js")
	_, _ = w.Write(b)
	b, _ = consoleFiles.ReadFile("workspace.js")
	_, _ = w.Write(append([]byte("\n"), b...))
	b, _ = consoleFiles.ReadFile("transfers.js")
	_, _ = w.Write(append([]byte("\n"), b...))
	b, _ = consoleFiles.ReadFile("clipboard.js")
	_, _ = w.Write(append([]byte("\n"), b...))
}

func (m *Module) console(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = consoleTemplate.Execute(w, u)
}
