package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/nathan-osman/cloudanchor/configurator"
	"github.com/nathan-osman/cloudanchor/server"
	"github.com/nathan-osman/cloudanchor/watcher"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "cloudanchor"
	app.Usage = "sync web server config with running Docker containers"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "acme-addr",
			Usage:  "`address` to listen on for ACME challenges",
			EnvVar: "ACME_ADDR",
			Value:  "127.0.0.1:8080",
		},
		cli.StringFlag{
			Name:   "acme-dir",
			Usage:  "`directory` for storing TLS keys and certs.",
			EnvVar: "ACME_DIR",
			Value:  "/etc/cloudanchor",
		},
		cli.StringFlag{
			Name:   "config-type",
			Usage:  "web server `type` to manage",
			EnvVar: "CONFIG_TYPE",
			Value:  configurator.Nginx,
		},
		cli.StringFlag{
			Name:   "config-file",
			Usage:  "`file` for storing web server configuration",
			EnvVar: "CONFIG_FILE",
		},
		cli.StringFlag{
			Name:   "config-pidfile",
			Usage:  "absolute `path` to pidfile for web server",
			EnvVar: "CONFIG_PIDFILE",
		},
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "enable debug output",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:   "docker-host",
			Usage:  "Docker engine `URI`",
			EnvVar: "DOCKER_HOST",
			Value:  "unix:///var/run/docker.sock",
		},
		cli.StringFlag{
			Name:   "server-addr",
			Usage:  "`address` for the admin server",
			EnvVar: "SERVER_ADDR",
		},
		cli.StringFlag{
			Name:   "server-username",
			Usage:  "`username` for HTTP basic auth",
			EnvVar: "SERVER_USERNAME",
		},
		cli.StringFlag{
			Name:   "server-password",
			Usage:  "`password` for HTTP basic auth",
			EnvVar: "SERVER_PASSWORD",
		},
	}
	app.Action = func(c *cli.Context) {

		if c.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		log := logrus.WithField("context", "main")

		// Create the configurator
		conf, err := configurator.New(
			context.TODO(),
			c.String("config-type"),
			c.String("config-file"),
			c.String("config-pidfile"),
			c.String("acme-addr"),
			c.String("acme-dir"),
		)
		if err != nil {
			log.Error(err)
			return
		}
		defer conf.Close()

		// Create the watcher
		watcher, err := watcher.New(conf, &watcher.Config{
			Host: c.String("docker-host"),
		})
		if err != nil {
			log.Error(err)
			return
		}
		defer watcher.Close()

		// Create the server
		srv, err := server.New(addr, username, password)
		if err != nil {
			log.Error(err)
			return
		}
		defer srv.Close()

		// Wait for a signal before shutting down
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
	}
	app.Run(os.Args)
}
