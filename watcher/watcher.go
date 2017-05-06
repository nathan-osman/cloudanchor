package watcher

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/nathan-osman/cloudanchor/configurator"
	"github.com/nathan-osman/cloudanchor/container"
	"github.com/sirupsen/logrus"
)

// Config stores the configuration for the watcher.
type Config struct {
	Host string
}

// Watcher monitors the Docker daemon for containers starting and stopping. The
// watcher will reconnect to the daemon if disconnected unexpectedly.
type Watcher struct {
	stopCh    chan bool
	stoppedCh chan bool
	conf      *configurator.Configurator
	log       *logrus.Entry
	client    *client.Client
}

// processContainers obtains a list of containers already running and adds them
// to the configurator.
func (w *Watcher) processContainers(ctx context.Context) error {
	containers, err := w.client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}
	containerList := []*container.Container{}
	for _, c := range containers {
		cJSON, err := w.client.ContainerInspect(ctx, c.ID)
		if err != nil {
			return err
		}
		if cont := container.New(cJSON); cont != nil {
			containerList = append(containerList, cont)
		}
	}
	return w.conf.Add(ctx, containerList...)
}

// addContainer adds a container to the configurator.
func (w *Watcher) addContainer(ctx context.Context, id string) error {
	cJSON, err := w.client.ContainerInspect(ctx, id)
	if err != nil {
		return err
	}
	if cont := container.New(cJSON); cont != nil {
		return w.conf.Add(ctx, cont)
	}
	return nil
}

// processEvents continues to process events from the client until an error
// occurs or the watcher is closed.
func (w *Watcher) processEvents(ctx context.Context) error {
	f := filters.NewArgs()
	f.Add("event", "start")
	f.Add("event", "die")
	msgChan, errChan := w.client.Events(ctx, types.EventsOptions{Filters: f})
	for {
		select {
		case m := <-msgChan:
			switch m.Action {
			case "start":
				w.log.Debugf("container %s started", m.ID)
				if err := w.addContainer(ctx, m.ID); err != nil {
					return err
				}
			case "die":
				w.log.Debugf("container %s died", m.ID)
				w.conf.Remove(ctx, m.ID)
			}
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return nil
		}
	}
}

// run maintains a persistent connection to the Docker daemon and watches for
// containers being started and stopped.
func (w *Watcher) run() {
	defer close(w.stoppedCh)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-w.stopCh
		cancel()
	}()
	for {
		var err error
		if err = w.processContainers(ctx); err != nil {
			goto retry
		}
		if err = w.processEvents(ctx); err != nil {
			goto retry
		}
		return
	retry:
		if err == context.Canceled {
			return
		}
		w.log.Error(err)
		select {
		case <-time.After(30 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// New creates a new Watcher and immediately begins the process of connecting
// to the Docker daemon.
func New(conf *configurator.Configurator, cfg *Config) (*Watcher, error) {
	c, err := client.NewClient(cfg.Host, "1.24", nil, nil)
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		stopCh:    make(chan bool),
		stoppedCh: make(chan bool),
		conf:      conf,
		log:       logrus.WithField("context", "watcher"),
		client:    c,
	}
	go w.run()
	return w, nil
}

// Close shuts down the watcher.
func (w *Watcher) Close() {
	close(w.stopCh)
	<-w.stoppedCh
}
