package config

import (
	"strings"
	"text/template"
)

var tmpl *template.Template

func init() {
	tmpl = template.New("nginx").Funcs(template.FuncMap{
		"join": strings.Join,
	})
	template.Must(tmpl.Parse(
		`# AUTO GENERATED FILE

{{range $c := .}}
# {{$c.Name}}

server {
    listen 80;
    listen [::]:80;
    server_name {{join $c.Domains " "}};
    return 301 https://{{index $c.Domains 0}};
}

server {
    listen 443 ssl;
    listen [::]:443;
    server_name {{join $c.Domains " "}};
    proxy_pass http://127.0.0.1:{{$c.Port}};
}
{{end}}`,
	))
}
