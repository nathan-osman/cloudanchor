package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/nathan-osman/cloudanchor/configurator"
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
			Name:   "server-type",
			Usage:  "web server `type` to manage",
			EnvVar: "SERVER_TYPE",
			Value:  configurator.Nginx,
		},
		cli.StringFlag{
			Name:   "server-file",
			Usage:  "`file` for storing web server configuration",
			EnvVar: "SERVER_FILE",
		},
		cli.StringFlag{
			Name:   "server-pidfile",
			Usage:  "absolute `path` to pidfile for web server",
			EnvVar: "SERVER_PIDFILE",
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
			c.String("server-type"),
			c.String("server-file"),
			c.String("server-pidfile"),
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

		// Wait for a signal before shutting down
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
	}
	app.Run(os.Args)
}
