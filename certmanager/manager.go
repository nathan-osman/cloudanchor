package certmanager

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme"
)

// Config stores the configuration for the certificate manager.
type Config struct {
	Directory string
	Port      int
}

// domainState keeps track of state for a single domain.
type domainState struct {
	active  bool
	domain  string
	expires time.Time
}

// CertManager manages TLS certificates for the currently running containers.
// Each domain requires a private key and x509 certificate signed by Let's
// Encrypt.
type CertManager struct {
	mutex       sync.Mutex
	renewCh     chan bool // manually trigger cert. renewal
	renewedCh   chan bool // renewal process completed
	stopCh      chan bool // stop the manager
	stoppedCh   chan bool // manager is stopped
	cfg         *Config
	log         *logrus.Entry
	client      *acme.Client
	states      map[string]*domainState
	nextTrigger time.Time
}

// run initializes the manager and renews certificates in a separate goroutine.
func (c *CertManager) run() {
	defer close(c.stoppedCh)
	defer close(c.renewedCh)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c.stopCh
		cancel()
	}()
	for {
		var (
			err    error
			timeCh <-chan time.Time
		)
		if err = c.initAccount(ctx); err != nil {
			goto retry
		}
		if !c.nextTrigger.IsZero() {
			timeCh = time.After(c.nextTrigger.Sub(time.Now()))
		}
		select {
		case <-timeCh:
		case <-c.renewCh:
		case <-c.stopCh:
			return
		}
		if err = c.renewExpiring(ctx); err != nil {
			goto retry
		}
		select {
		case c.renewedCh <- true:
		default:
		}
		continue
	retry:
		if err == context.Canceled {
			return
		}
		c.log.Error(err)
		select {
		case <-time.After(30 * time.Second):
		case <-c.stopCh:
			return
		}
	}
}

// New creates a new certificate manager.
func New(cfg *Config) (*CertManager, error) {
	c := &CertManager{
		renewCh:   make(chan bool),
		renewedCh: make(chan bool),
		stopCh:    make(chan bool),
		stoppedCh: make(chan bool),
		cfg:       cfg,
		log:       logrus.WithField("context", "certmanager"),
		client: &acme.Client{
			// TODO: remove this before release
			DirectoryURL: "https://acme-staging.api.letsencrypt.org/directory",
		},
		states: make(map[string]*domainState),
	}
	go c.run()
	return c, nil
}

// Add indicates that the specified domain name is going to be used. The
// Apply() method can be used to perform the actual renewal.
func (c *CertManager) Add(domain string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	state, ok := c.states[domain]
	if ok {
		state.active = true
	} else {
		c.states[domain] = &domainState{
			active: true,
			domain: domain,
		}
	}
}

// Remove indicates that the provided domain name is no longer in use. The
// domain is marked as inactive and it is no longer renewed.
func (c *CertManager) Remove(domain string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	state, ok := c.states[domain]
	if ok {
		state.active = false
	}
}

// Apply performs all of the steps necessary to ensure that all pending
// certificate requests complete and blocks until the certificates are renewed.
func (c *CertManager) Apply() {
	c.renewCh <- true
	<-c.renewedCh
}

// Close instructs the certificate manager to shut down.
func (c *CertManager) Close() {
	close(c.stopCh)
	<-c.stoppedCh
}
