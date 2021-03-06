package configurator

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nathan-osman/cloudanchor/container"
	"github.com/nathan-osman/go-simpleacme/manager"
	"github.com/sirupsen/logrus"
)

const (
	Apache = "apache"
	Nginx  = "nginx"
)

// Configurator updates server configuration changes when containers are
// started and stopped.
type Configurator struct {
	mutex      sync.Mutex
	stop       chan bool
	stopped    chan bool
	add        chan []*container.Container
	remove     chan string
	type_      string
	file       string
	pidfile    string
	addr       string
	dir        string
	log        *logrus.Entry
	mgr        *manager.Manager
	containers map[string]*container.Container
	tlsDomains map[string]bool
}

// reload instructs the server to reload its configuration.
func (c *Configurator) reload() error {
	b, err := ioutil.ReadFile(c.pidfile)
	if err != nil {
		return fmt.Errorf("unable to open pidfile: %s", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid == 0 {
		return errors.New("unable to read pidfile")
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("unable to reload: %s", err)
	}
	return nil
}

// writeConfig writes the server configuration to disk.
func (c *Configurator) writeConfig() error {
	c.log.Debugf("writing %s configuration file", c.type_)
	w, err := os.Create(c.file)
	if err != nil {
		return err
	}
	defer w.Close()
	tmpls := []*domainTmpl{}
	func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		for _, cont := range c.containers {
			for _, d := range cont.Domains {
				tmpls = append(tmpls, &domainTmpl{
					Name:      d,
					Addr:      cont.Addr,
					Key:       c.mgr.Key(d),
					Cert:      c.mgr.Cert(d),
					AuthAddr:  c.addr,
					EnableTLS: c.tlsDomains[d],
				})
			}
		}
	}()
	if err := tmpl.ExecuteTemplate(w, c.type_, tmpls); err != nil {
		return err
	}
	return c.reload()
}

// callback updates the config file and triggers a server reload.
func (c *Configurator) callback(domains ...string) {
	c.log.Debugf("certificates loaded for %d domain(s)", len(domains))
	func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		for _, d := range domains {
			c.tlsDomains[d] = true
		}
	}()
	if err := c.writeConfig(); err != nil {
		c.log.Error(err)
	}
}

// addContainers adds containers to the configurator.
func (c *Configurator) addContainers(ctx context.Context, containers ...*container.Container) error {
	domains := []string{}
	func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		for _, cont := range containers {
			c.containers[cont.ID] = cont
			domains = append(domains, cont.Domains...)
		}
	}()
	if err := c.writeConfig(); err != nil {
		return err
	}
	go func() {
		select {
		case <-time.After(2 * time.Second):
			c.mgr.Add(ctx, domains...)
		case <-ctx.Done():
		}
	}()
	return nil
}

// removeContainer removes a container from the configurator.
func (c *Configurator) removeContainer(ctx context.Context, id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if cont, ok := c.containers[id]; ok {
		for _, d := range cont.Domains {
			delete(c.tlsDomains, d)
		}
		delete(c.containers, id)
		if err := c.writeConfig(); err != nil {
			return err
		}
		if err := c.mgr.Remove(ctx, cont.Domains...); err != nil {
			return err
		}
	}
	return nil
}

// run responds to container changes and restarts the server.
func (c *Configurator) run() {
	defer close(c.stopped)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c.stop
		cancel()
	}()
	for {
		var err error
		select {
		case containers := <-c.add:
			err = c.addContainers(ctx, containers...)
		case id := <-c.remove:
			err = c.removeContainer(ctx, id)
		case <-ctx.Done():
			return
		}
		if err == context.Canceled {
			return
		}
		if err != nil {
			c.log.Error(err)
		}
	}
}

// New creates a new configurator instance.
func New(ctx context.Context, type_, file, pidfile, addr, dir string) (*Configurator, error) {
	switch type_ {
	case Nginx:
		if len(file) == 0 {
			file = "/etc/nginx/sites-enabled/cloudanchor.conf"
		}
		if len(pidfile) == 0 {
			pidfile = "/var/run/nginx.pid"
		}
	default:
		return nil, fmt.Errorf("unrecognized server type \"%s\"", type_)
	}
	c := &Configurator{
		stop:       make(chan bool),
		stopped:    make(chan bool),
		add:        make(chan []*container.Container),
		remove:     make(chan string),
		type_:      type_,
		file:       file,
		pidfile:    pidfile,
		addr:       addr,
		dir:        dir,
		log:        logrus.WithField("context", "configurator"),
		containers: make(map[string]*container.Container),
		tlsDomains: make(map[string]bool),
	}
	m, err := manager.New(ctx, addr, dir, c.callback)
	if err != nil {
		return nil, err
	}
	c.mgr = m
	go c.run()
	return c, nil
}

// Containers retrieves the list of currently registered containers.
func (c *Configurator) Containers() []*container.Container {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	containers := []*container.Container{}
	for _, cont := range c.containers {
		containers = append(containers, cont)
	}
	return containers
}

// Add adds containers to the configurator.
func (c *Configurator) Add(ctx context.Context, containers ...*container.Container) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.add <- containers:
		return nil
	}
}

// Remove removes a container from the configurator.
func (c *Configurator) Remove(ctx context.Context, id string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.remove <- id:
		return nil
	}
}

// Close shuts down the configurator.
func (c *Configurator) Close() {
	close(c.stop)
	<-c.stopped
	c.mgr.Close()
}
