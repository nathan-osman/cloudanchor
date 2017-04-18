package configurator

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
	"github.com/nathan-osman/cloudanchor/container"
)

const (
	Nginx = "nginx"
)

// Config stores the configuration for the configurator.
type Config struct {
	Type    string `json:"type"`
	File    string `json:"file"`
	Pidfile string `json:"pidfile"`
}

// Configurator maintains a list of running containers and generates
// the appropriate web server configuration file on-demand.
type Configurator struct {
	mutex      sync.Mutex
	stop       chan bool
	containers map[string]*container.Container
	cfg        *Config
	log        *logrus.Entry
}

// writeTemplate writes the template to disk.
func (c *Configurator) writeTemplate() error {
	w, err := os.Create(c.cfg.File)
	if err != nil {
		return err
	}
	defer w.Close()
	return tmpl.ExecuteTemplate(w, c.cfg.Type, c.Containers())
}

// reload attempts to reload the web server configuration.
func (c *Configurator) reload() error {
	b, err := ioutil.ReadFile(c.cfg.Pidfile)
	if err != nil {
		return fmt.Errorf("unable to read pidfile: %s", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid == 0 {
		return errors.New("pidfile is corrupt")
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("unable to restart %s: %s", c.cfg.Type, err)
	}
	return nil
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
	return &Configurator{
		stop:       make(chan bool),
		containers: make(map[string]*container.Container),
		cfg:        cfg,
		log:        logrus.WithField("context", "config"),
	}, nil
}

// Add adds the container to the list of those managed by the web server. The
// changes are not applied until the Reload() method is invoked. This method is
// thread-safe.
func (c *Configurator) Add(cont *container.Container) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.containers[cont.ID] = cont
}

// Remove removes the container from the list of those managed by the web
// server. The changes are not applied until the Reload() method is invoked.
// This method is thread-safe.
func (c *Configurator) Remove(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.containers, id)
}

// Containers returns a list of all running containers. This method is
// thread-safe.
func (c *Configurator) Containers() []*container.Container {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var (
		containers = make([]*container.Container, len(c.containers))
		i          = 0
	)
	for _, cont := range c.containers {
		containers[i] = cont
		i += 1
	}
	return containers
}

// Reload generates the configuration file for the web server and applies it.
// This method is thread-safe.
func (c *Configurator) Reload() {
	if err := c.writeTemplate(); err != nil {
		c.log.Error(err)
	}
	if err := c.reload(); err != nil {
		c.log.Error(err)
	}
}
