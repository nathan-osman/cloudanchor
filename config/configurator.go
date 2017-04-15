package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
	Type      string `json:"type"`
	Directory string `json:"directory"`
	Pidfile   string `json:"pidfile"`

	ContainerStarted <-chan *watcher.Container
	ContainerStopped <-chan *watcher.Container
	Reload           <-chan bool
}

// Configurator listens for the addition or removal of Docker containers and
// adjusts the configuration for the managed web server.
type Configurator struct {
	cfg *Config
	log *logrus.Entry
	wg  sync.WaitGroup
}

// filename generates the absolute path to the configuration file for the
// specified container.
func (c *Configurator) filename(container *watcher.Container) string {
	return path.Join(c.cfg.Directory, fmt.Sprintf("%s.conf", container.Name))
}

// createConfig creates a configuration file for the specified container.
func (c *Configurator) createConfig(container *watcher.Container) error {
	w, err := os.Create(c.filename(container))
	if err != nil {
		return err
	}
	defer w.Close()
	return nginxTemplate.Execute(w, map[string]interface{}{
		"domains": strings.Join(container.Domains, " "),
		"domain":  container.Domains[0],
		"port":    container.Port,
	})
}

// runStart generates configuration files for containers.
func (c *Configurator) runStart() {
	defer c.wg.Done()
	for container := range c.cfg.ContainerStarted {
		if err := c.createConfig(container); err != nil {
			c.log.Error(err)
		}
	}
}

// runStop removes configuration files for containers.
func (c *Configurator) runStop() {
	defer c.wg.Done()
	for container := range c.cfg.ContainerStopped {
		if err := os.Remove(c.filename(container)); err != nil {
			c.log.Error(err)
		}
	}
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

// runReload reloads the server configuration upon request.
func (c *Configurator) runReload() {
	defer c.wg.Done()
	for _ = range c.cfg.Reload {
		if err := c.reload(); err != nil {
			c.log.Error(err)
		}
	}
}

// New creates a new configurator using the provided configuration. Be sure to
// call the Wait() method on the returned configurator when cleaning up.
func New(cfg *Config) (*Configurator, error) {
	switch cfg.Type {
	case Nginx:
		if len(cfg.Directory) == 0 {
			cfg.Directory = "/etc/nginx/sites-enabled"
		}
		if len(cfg.Pidfile) == 0 {
			cfg.Pidfile = "/var/run/nginx.pid"
		}
	default:
		return nil, fmt.Errorf("unrecognized server type \"%s\"", cfg.Type)
	}
	if _, err := os.Stat(cfg.Directory); err != nil {
		return nil, err
	}
	c := &Configurator{
		cfg: cfg,
		log: logrus.WithField("context", "watcher"),
	}
	c.wg.Add(3)
	go c.runStart()
	go c.runStop()
	go c.runReload()
	return c, nil
}

// Wait blocks until the configurator has finished shutting down.
func (c *Configurator) Wait() {
	c.wg.Wait()
}
