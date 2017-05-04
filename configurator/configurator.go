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

type Configurator struct {
	mutex      sync.Mutex
	stop       chan bool
	stopped    chan bool
	add        chan *container.Container
	remove     chan string
	type_      string
	file       string
	pidfile    string
	addr       string
	dir        string
	log        *logrus.Entry
	mgr        *manager.Manager
	containers map[string]*container.Container
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
func (c *Configurator) writeConfig(enableTLS bool) error {
	w, err := os.Create(c.file)
	if err != nil {
		return err
	}
	defer w.Close()
	tmpls := []*domainTmpl{}
	for _, cont := range c.containers {
		for _, d := range cont.Domains {
			tmpls = append(tmpls, &domainTmpl{
				Name:      d,
				Addr:      cont.Addr,
				Key:       c.mgr.Key(d),
				Cert:      c.mgr.Cert(d),
				AuthAddr:  c.addr,
				EnableTLS: enableTLS,
			})
		}
	}
	if err := tmpl.ExecuteTemplate(w, c.type_, tmpls); err != nil {
		return err
	}
	return c.reload()
}

// callback updates the config file and triggers a server reload.
func (c *Configurator) callback(...string) error {
	return c.writeConfig(true)
}

// run responds to container changes and restarts the server.
func (c *Configurator) run() {
	defer close(c.stopped)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c.stop
		cancel()
	}()
	var (
		pendingContainers = make(map[string]*container.Container)
		pendingTrigger    <-chan time.Time
	)
	for {
		select {
		case cont := <-c.add:
			pendingContainers[cont.ID] = cont
			pendingTrigger = time.After(10 * time.Second)
		case id := <-c.remove:
			delete(pendingContainers, id)
			func() {
				c.mutex.Lock()
				defer c.mutex.Unlock()
				delete(c.containers, id)
			}()
		case <-pendingTrigger:
			domains := []string{}
			func() {
				c.mutex.Lock()
				defer c.mutex.Unlock()
				for _, cont := range pendingContainers {
					domains = append(domains, cont.Domains...)
					c.containers[cont.ID] = cont
				}
			}()
			if err := c.writeConfig(false); err != nil {
				c.log.Error(err)
				continue
			}
			go func() {
				c.mgr.Add(ctx, domains...)
			}()
			pendingContainers = make(map[string]*container.Container)
			pendingTrigger = nil
		case <-ctx.Done():
			return
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
		add:        make(chan *container.Container),
		remove:     make(chan string),
		type_:      type_,
		file:       file,
		pidfile:    pidfile,
		addr:       addr,
		dir:        dir,
		log:        logrus.WithField("context", "configurator"),
		containers: make(map[string]*container.Container),
	}
	m, err := manager.New(ctx, addr, dir, c.callback)
	if err != nil {
		return nil, err
	}
	c.mgr = m
	go c.run()
	return c, nil
}

// Add adds a domain to the configurator.
func (c *Configurator) Add(ctx context.Context, cont *container.Container) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.add <- cont:
		return nil
	}
}

// Remove removes a domain from the configurator.
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
