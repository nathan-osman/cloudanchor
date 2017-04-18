package watcher

import (
	"context"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/nathan-osman/cloudanchor/configurator"
	"github.com/nathan-osman/cloudanchor/container"
)

// Config stores the configuration for the watcher.
type Config struct {
	Host string
}

// Watcher monitors the Docker daemon for containers starting and stopping. The
// watcher will reconnect to the daemon if disconnected unexpectedly.
type Watcher struct {
	stop chan bool
	conf *configurator.Configurator
	cfg  *Config
	log  *logrus.Entry
}

// processContainers obtains a list of containers already running and adds them
// to the configurator.
func (w *Watcher) processContainers(dClient *client.Client) error {
	dContainers, err := dClient.ContainerList(context.TODO(), types.ContainerListOptions{})
	if err != nil {
		return err
	}
	for _, c := range dContainers {
		if cont := container.New(c.ID, c.Labels); cont != nil {
			w.conf.Add(cont)
		}
	}
	w.conf.Reload()
	return nil
}

// processEvents continues to process events from the client until an error
// occurs or the watcher is closed. The return value is true if the watcher
// should reconnect.
func (w *Watcher) processEvents(dClient *client.Client) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := filters.NewArgs()
	f.Add("event", "start")
	f.Add("event", "die")
	msgChan, errChan := dClient.Events(ctx, types.EventsOptions{Filters: f})
	for {
		select {
		case m := <-msgChan:
			switch m.Action {
			case "start":
				cJSON, err := dClient.ContainerInspect(context.TODO(), m.ID)
				if err != nil {
					w.log.Warning(err)
					continue
				}
				if cont := container.New(cJSON.ID, cJSON.Config.Labels); cont != nil {
					w.conf.Add(cont)
					w.conf.Reload()
				}
			case "die":
				w.conf.Remove(m.ID)
				w.conf.Reload()
			}
		case err := <-errChan:
			w.log.Error(err)
			return true
		case <-w.stop:
			return false
		}
	}
}

// run maintains a persistent connection to the Docker daemon and watches for
// containers being started and stopped.
func (w *Watcher) run() {
	defer close(w.stop)
	for {
		c, err := client.NewClient(w.cfg.Host, "1.24", nil, nil)
		if err != nil {
			w.log.Error(err)
			goto error
		}
		w.log.Info("connected to daemon")
		if err := w.processContainers(c); err != nil {
			w.log.Error(err)
			goto error
		}
		if !w.processEvents(c) {
			c.Close()
			return
		}
	error:
		w.log.Info("reconnecting in 30 seconds")
		select {
		case <-time.After(30 * time.Second):
		case <-w.stop:
			return
		}
	}
}

// New creates a new Watcher and immediately begins the process of connecting
// to the Docker daemon.
func New(conf *configurator.Configurator, cfg *Config) *Watcher {
	w := &Watcher{
		stop: make(chan bool),
		conf: conf,
		cfg:  cfg,
		log:  logrus.WithField("context", "watcher"),
	}
	go w.run()
	return w
}

// Close shuts down the watcher.
func (w *Watcher) Close() {
	w.stop <- true
	<-w.stop
}
