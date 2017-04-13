package cloudanchor

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/client"
)

// Watcher monitors the Docker daemon for containers starting and stopping. The
// watcher will reconnect to the daemon if disconnected unexpectedly.
type Watcher struct {
	ContainerStarted <-chan *Container
	ContainerStopped <-chan string
	containerStarted chan<- *Container
	containerStopped chan<- string
	stop             chan bool
	stopped          chan bool
	log              *logrus.Entry
}

// run maintains a persistent connection to the Docker daemon and watches for
// containers being started and stopped.
func (w *Watcher) run() {
	defer close(w.stopped)
	for {
		c, err := client.NewEnvClient()
		if err != nil {
			w.log.Error(err)
			goto error
		}
		//...
	error:
		w.log.Info("reconnecting in 30 seconds")
		select {
		case <-time.After(30 * time.Second):
		case <-w.stop:
			c.Close()
			return
		}
	}
}

// NewWatcher creates a new Watcher and immediately begins the process of
// connecting to the Docker daemon.
func NewWatcher() *Watcher {
	var (
		containerStarted = make(chan *Container)
		containerStopped = make(chan string)
		w                = &Watcher{
			ContainerStarted: containerStarted,
			ContainerStopped: containerStopped,
			containerStarted: containerStarted,
			containerStopped: containerStopped,
			stop:             make(chan bool),
			stopped:          make(chan bool),
			log:              logrus.WithField("context", "watcher"),
		}
	)
	go w.run()
	return w
}

// Close shuts down the watcher.
func (w *Watcher) Close() {
	close(w.stop)
	<-w.stopped
}
