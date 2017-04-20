package certmanager

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
)

// generateKey creates a new RSA key.
func (c *CertManager) generateKey() (crypto.Signer, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// writeKeys writes an RSA key to disk.
func (c *CertManager) writeKeys(key crypto.Signer, domains ...string) error {
	b := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	for _, d := range domains {
		if err = ioutil.WriteFile(c.filename(d, typeKey), b, 0600); err != nil {
			return err
		}
	}
	return nil
}
