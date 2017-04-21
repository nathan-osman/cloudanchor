package certmanager

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	stateInactive = iota
	stateActive
	statePending  // cert marked for renewal but not started
	stateRenewing // cert currently being renewed (transition to active)
	stateCanceled // cert renewal canceled (transition to inactive)
)

const (
	day  = 24 * time.Hour
	week = 7 * day
)

var errInvalidCert = errors.New("invalid certificate")

// domainState keeps track of state for a single domain.
type domainState struct {
	sync.Mutex
	current int
	domain  string
	expires time.Time
}

// loadCert attempts to load a certificate for the specified domain. Basic
// sanity checks are performed to ensure a private key is available and the
// certificate has not expired.
func (c *CertManager) loadCert(domain string) (*domainState, error) {
	if _, err := os.Stat(c.Filename(domain, TypeKey)); err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(c.Filename(domain, TypeCert))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errInvalidCert
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	if time.Now().After(cert.NotAfter) {
		return nil, errInvalidCert
	}
	return &domainState{
		domain:  domain,
		expires: cert.NotAfter,
	}, nil
}

// loadCerts parses the certificates in the directory. Any that are invalid are
// removed to help keep things tidy.
func (c *CertManager) loadCerts() error {
	files, err := ioutil.ReadDir(c.cfg.Directory)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), "."+TypeCert) {
			continue
		}
		domain, err := c.domain(f.Name())
		if err != nil {
			continue
		}
		state, err := c.loadCert(domain)
		if err != nil {
			os.Remove(c.Filename(domain, TypeKey))
			os.Remove(c.Filename(domain, TypeCert))
			continue
		}
		c.domains[domain] = state
	}
	return nil
}

// pendingDomains returns a list of all domains whose certificates are within
// the expiry threshold. Every matching cert is marked for renewal.
func (c *CertManager) pendingDomains() []*domainState {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	domains := []*domainState{}
	for _, s := range c.domains {
		s.Lock()
		if s.current == stateActive && time.Now().Add(2*week).After(s.expires) ||
			s.current == statePending {
			s.current = statePending
			domains = append(domains, s)
		}
		s.Unlock()
	}
	return domains
}
