package configurator

import (
	"text/template"
)

var tmpl *template.Template

// domainTmpl provides the template with the information it needs for a
// container.
type domainTmpl struct {
	Name      string
	Port      int
	Key       string
	Cert      string
	Addr      string
	EnableTLS bool
}

func init() {
	tmpl = template.New("nginx")
	template.Must(tmpl.Parse(
		`# AUTO GENERATED FILE

{{range $d := .}}
# {{$d.Name}}
server {
    listen 80;
    listen [::]:80;
    server_name {{$d.Name}};
{{if $d.EnableTLS}}
    location / {
        return 301 https://{{$d.Name}};
    }
{{else}}
    location /.well-known {
        proxy_pass http://{{$d.Addr}};
    }
{{end}}
}
{{if $d.EnableTLS}}
server {
    listen 443 ssl;
    listen [::]:443;
    server_name {{$d.Name}};

    location / {
        proxy_pass http://127.0.0.1:{{$d.Port}};
    }

    ssl on;
    ssl_certificate {{$d.Cert}};
    ssl_certificate_key {{$d.Key}};
}
{{end}}
{{end}}`,
	))
}
