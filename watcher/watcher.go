package watcher

import (
	"context"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Watcher monitors the Docker daemon for containers starting and stopping. The
// watcher will reconnect to the daemon if disconnected unexpectedly.
type Watcher struct {
	containerStarted  chan<- *Container
	containerStopped  chan<- *Container
	runningContainers map[string]*Container
	stop              chan bool
	stopped           chan bool
	log               *logrus.Entry
}

// processStart creates a new Container instance and
func (w *Watcher) processStart(j types.ContainerJSON) {
	if container := newContainer(j.ID, j.Config.Labels); container != nil {
		w.runningContainers[container.ID] = container
		w.containerStarted <- container
	}
}

// processDie sends on the containerStopped channel to indicate that the
// specified container is no longer running.
func (w *Watcher) processDie(id string) {
	if container, ok := w.runningContainers[id]; ok {
		w.containerStopped <- container
		delete(w.runningContainers, id)
	}
}

// processContainers obtains a list of containers already running.
func (w *Watcher) processContainers(c *client.Client) error {
	containers, err := c.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return err
	}
	for _, c := range containers {
		if container := newContainer(c.ID, c.Labels); container != nil {
			w.runningContainers[container.ID] = container
			w.containerStarted <- container
		}
	}
	return nil
}

// runEvents continues to process events from the client until an error occurs
// or the watcher is closed. The return value is true if the watcher should
// reconnect.
func (w *Watcher) runEvents(c *client.Client) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := filters.NewArgs()
	f.Add("event", "start")
	f.Add("event", "die")
	msgChan, errChan := c.Events(ctx, types.EventsOptions{Filters: f})
	for {
		select {
		case m := <-msgChan:
			switch m.Action {
			case "start":
				j, err := c.ContainerInspect(ctx, m.ID)
				if err != nil {
					w.log.Warning(err)
					continue
				}
				w.processStart(j)
			case "die":
				w.processDie(m.ID)
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
	defer close(w.stopped)
	defer close(w.containerStopped)
	defer close(w.containerStarted)
	for {
		w.runningContainers = make(map[string]*Container)
		c, err := client.NewEnvClient()
		if err != nil {
			w.log.Error(err)
			goto error
		}
		w.log.Info("connected to daemon")
		if err := w.processContainers(c); err != nil {
			w.log.Error(err)
			goto error
		}
		if !w.runEvents(c) {
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

// NewWatcher creates a new Watcher and immediately begins the process of
// connecting to the Docker daemon.
func NewWatcher(containerStarted chan<- *Container, containerStopped chan<- *Container) *Watcher {
	w := &Watcher{
		containerStarted: containerStarted,
		containerStopped: containerStopped,
		stop:             make(chan bool),
		stopped:          make(chan bool),
		log:              logrus.WithField("context", "watcher"),
	}
	go w.run()
	return w
}

// Close shuts down the watcher.
func (w *Watcher) Close() {
	close(w.stop)
	<-w.stopped
}
