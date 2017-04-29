package certmanager

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

var errInvalidKey = errors.New("invalid private key")

// loadKey attempts to load the RSA key for a domain from disk.
func (c *CertManager) loadKey(domain string) (*rsa.PrivateKey, error) {
	b, err := ioutil.ReadFile(c.Filename(domain, TypeKey))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, errInvalidKey
	}
	k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return k, nil
}

// generateKey creates a new RSA key.
func (c *CertManager) generateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// writeKeys writes an RSA key to disk.
func (c *CertManager) writeKeys(key *rsa.PrivateKey, domains ...string) error {
	b := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	for _, d := range domains {
		if err := ioutil.WriteFile(c.Filename(d, TypeKey), b, 0600); err != nil {
			return err
		}
	}
	return nil
}
