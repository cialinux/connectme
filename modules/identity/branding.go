package identity

import (
	"html/template"
	"strings"
)

// ReleaseVersion is set from the image release tag by the build workflow.
var ReleaseVersion = "1.0.16"

const releaseFooter = `<footer style="padding:24px;text-align:center;font-size:14px"><a style="color:#8cbaff" href="https://github.com/cialinux/connectme">versão {{releaseVersion}}</a> - <a style="color:#8cbaff" href="https://cialinux.com">cialinux</a></footer>`

func brandedTemplate(name, source string) *template.Template {
	source = strings.ReplaceAll(source, "<h1>ConnectMe</h1>", "<h1>ConnectMe by cialinux</h1>")
	source = strings.Replace(source, "</body>", releaseFooter+"</body>", 1)
	return template.Must(template.New(name).Funcs(template.FuncMap{
		"releaseVersion": func() string { return ReleaseVersion },
	}).Parse(source))
}
