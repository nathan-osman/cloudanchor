package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/nathan-osman/cloudanchor/watcher"
)

const (
	Nginx = "nginx"
)

// Config stores the configuration for the configurator.
type Config struct {
	Type    string `json:"type"`
	File    string `json:"file"`
	Pidfile string `json:"pidfile"`

	ContainerStarted <-chan *watcher.Container
	ContainerStopped <-chan string
	Reload           <-chan bool
}

// Configurator listens for the addition or removal of Docker containers and
// adjusts the configuration for the managed web server.
type Configurator struct {
	mutex sync.Mutex
	stop  chan bool
	cList map[string]*watcher.Container
	cfg   *Config
	log   *logrus.Entry
}

// writeTemplate writes the template to disk.
func (c *Configurator) writeTemplate() error {
	w, err := os.Create(c.cfg.File)
	if err != nil {
		return err
	}
	defer w.Close()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return tmpl.ExecuteTemplate(w, c.cfg.Type, c.cList)
}

// reload sends the SIGHUP signal to the web server process to inform it that
// the configuration has changed.
func (c *Configurator) reload() error {
	b, err := ioutil.ReadFile(c.cfg.Pidfile)
	if err != nil {
		return err
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid == 0 {
		return errors.New("unable to read PID")
	}
	return syscall.Kill(pid, syscall.SIGHUP)
}

// run processes events as they are received on the channels
func (c *Configurator) run() {
	defer close(c.stop)
	for {
		select {
		case container := <-c.cfg.ContainerStarted:
			c.mutex.Lock()
			c.cList[container.ID] = container
			c.mutex.Unlock()
		case id := <-c.cfg.ContainerStopped:
			c.mutex.Lock()
			delete(c.cList, id)
			c.mutex.Unlock()
		case <-c.cfg.Reload:
			if err := c.writeTemplate(); err != nil {
				c.log.Error(err)
			}
			if err := c.reload(); err != nil {
				c.log.Error(err)
			}
		case <-c.stop:
			return
		}
	}
}

// New creates a new configurator using the provided configuration.
func New(cfg *Config) (*Configurator, error) {
	switch cfg.Type {
	case Nginx:
		if len(cfg.File) == 0 {
			cfg.File = "/etc/nginx/sites-enabled/cloudanchor.conf"
		}
		if len(cfg.Pidfile) == 0 {
			cfg.Pidfile = "/var/run/nginx.pid"
		}
	default:
		return nil, fmt.Errorf("unrecognized server type \"%s\"", cfg.Type)
	}
	c := &Configurator{
		stop:  make(chan bool),
		cList: make(map[string]*watcher.Container),
		cfg:   cfg,
		log:   logrus.WithField("context", "config"),
	}
	go c.run()
	return c, nil
}

// Close shuts down the configurator.
func (c *Configurator) Close() {
	c.stop <- true
	<-c.stop
}
