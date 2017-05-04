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

	// Nginx
	template.Must(tmpl.Parse(
		`# AUTO GENERATED FILE

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

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
    location /.well-known/ {
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
        proxy_http_version 1.1
        proxy_set_header Host             $host;
        proxy_set_header X-Real-IP        $remote_addr;
        proxy_set_header X-Forwarded-For  $proxy_add_x_forwarded_for
        proxy_set_header Upgrade          $http_upgrade;
        proxy_set_header Connection       $connection_upgrade;
    }

    ssl on;
    ssl_certificate {{$d.Cert}};
    ssl_certificate_key {{$d.Key}};
}
{{end}}
{{end}}`,
	))

	// Apache
	template.Must(tmpl.New("apache").Parse(
		`#AUTO GENERATED FILE

{{range $d := .}}
<VirtualHost *:80>
    ServerName {{$d.Name}}
{{if $d.EnableTLS}}
    Redirect permanent / https://{{$d.Name}}/
{{else}}
    ProxyPreserveHost On
    ProxyPass /.well-known/ http://{{$d.Addr}}/.well-known/
{{end}}
</VirtualHost>
{{if $d.EnableTLS}}
<VirtualHost *:443>
    ServerName {{$d.Name}}

    ProxyPreserveHost On
    ProxyPass / http://{{$d.Addr}}/

    SSLEngine On
    SSLCertificateFile {{$d.Cert}}
    SSLCertificateKeyFile {{$d.Key}}
</VirtualHost>
{{end}}
{{end}}
        `,
	))
}
