package certmanager

import (
	"time"

	"github.com/Sirupsen/logrus"
	"golang.org/x/crypto/acme"
)

// Config stores the configuration for the certificate manager.
type Config struct {
	Directory string
	Port      int
}

// CertManager manages TLS certificates for the currently running containers.
// Each domain requires a private key and x509 certificate signed by Let's
// Encrypt.
type CertManager struct {
	stop        chan bool
	stopped     chan bool
	cfg         *Config
	log         *logrus.Entry
	client      *acme.Client
	tasks       map[string]*renewalTask
	nextTrigger time.Time
}

// TODO
func (c *CertManager) run() {
	defer close(c.stopped)
	for {
		select {
		case <-c.stop:
			return
		}
	}
}

// New creates a new certificate manager.
func New(cfg *Config) (*CertManager, error) {
	c := &CertManager{
		stop:    make(chan bool),
		stopped: make(chan bool),
		cfg:     cfg,
		log:     logrus.WithField("context", "certmanager"),
		client:  &acme.Client{},
		tasks:   make(map[string]*renewalTask),
	}
	go c.run()
	return c, nil
}

// Close instructs the certificate manager to shut down.
func (c *CertManager) Close() {
	close(c.stop)
	<-c.stopped
}
