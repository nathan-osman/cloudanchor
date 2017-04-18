package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/nathan-osman/cloudanchor/configurator"
	"github.com/nathan-osman/cloudanchor/watcher"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "cloudanchor"
	app.Usage = "sync web server config with running Docker containers"
	app.Flags = []cli.Flag{
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
			Value:  "nginx",
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

		log := logrus.WithField("context", "main")

		// Create the configurator
		conf, err := configurator.New(&configurator.Config{
			Type:    c.String("server-type"),
			File:    c.String("server-file"),
			Pidfile: c.String("server-pidfile"),
		})
		if err != nil {
			log.Error(err)
			return
		}

		// Create the watcher
		watcher := watcher.New(conf, &watcher.Config{
			Host: c.String("docker-host"),
		})
		defer watcher.Close()

		// Wait for a signal before shutting down
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
	}
	app.Run(os.Args)
}
