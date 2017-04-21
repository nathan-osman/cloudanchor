package certmanager

import (
	"context"
	"sync"
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
	mutex       sync.Mutex
	stop        chan bool
	stopped     chan bool
	cfg         *Config
	log         *logrus.Entry
	client      *acme.Client
	domains     map[string]*domainState
	nextTrigger time.Time
}

// run renews certificates as they are added and monitors them for expiry.
func (c *CertManager) run() {
	defer close(c.stopped)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		select {
		case <-ctx.Done():
		case <-c.stop:
			cancel()
		}
	}()
	for {
		if err := c.initAccount(ctx); err != nil {
			if err == context.Canceled {
				return
			}
			c.log.Error(err)
			goto retry
		}
		select {
		case <-c.stop:
			return
		}
	retry:
		select {
		case <-time.After(30 * time.Second):
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
		client: &acme.Client{
			DirectoryURL: "https://acme-staging.api.letsencrypt.org/directory",
		},
		domains: make(map[string]*domainState),
	}
	go c.run()
	return c, nil
}

// Add indicates that the specified domain name is going to be used. The
// Apply() method can be used to perform the actual renewal.
func (c *CertManager) Add(domain string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	state, ok := c.domains[domain]
	if ok {
		state.Lock()
		defer state.Unlock()
		switch state.current {
		case stateInactive:
			state.current = stateActive
		case stateCanceled:
			state.current = stateRenewing
		}
	} else {
		c.domains[domain] = &domainState{
			current: statePending,
			domain:  domain,
		}
	}
}

// Remove indicates that the provided domain name is no longer in use. The
// domain is marked as inactive and it is no longer renewed.
func (c *CertManager) Remove(domain string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	state, ok := c.domains[domain]
	if ok {
		state.Lock()
		defer state.Unlock()
		switch state.current {
		case stateActive:
		case statePending:
			state.current = stateInactive
		case stateRenewing:
			state.current = stateCanceled
		}
	}
}

// Apply performs all of the steps necessary to ensure that all pending
// certificate requests complete and blocks until the certificates are renewed,
// an error occurs, or the context is canceled.
func (c *CertManager) Apply(ctx context.Context) error {
	return c.renewPending(ctx)
}

// Close instructs the certificate manager to shut down.
func (c *CertManager) Close() {
	close(c.stop)
	<-c.stopped
}
