package certmanager

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

var errInvalidCert = errors.New("invalid certificate")

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
// removed (along with any private key) to help keep things tidy.
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
			if err == errInvalidCert {
				os.Remove(c.Filename(domain, TypeKey))
				os.Remove(c.Filename(domain, TypeCert))
			}
			continue
		}
		c.states[domain] = state
	}
	return nil
}
