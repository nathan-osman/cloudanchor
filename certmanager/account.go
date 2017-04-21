package certmanager

import (
	"context"
	"crypto/rsa"
	"os"

	"golang.org/x/crypto/acme"
)

// register prepares the account for use by generating a new private key. The
// account URI is written to a JSON file.
func (c *CertManager) register(ctx context.Context) (*rsa.PrivateKey, error) {
	k, err := c.generateKey()
	if err != nil {
		return nil, err
	}
	_, err = c.client.Register(ctx, nil, acme.AcceptTOS)
	if err != nil {
		return nil, err
	}
	if err := c.writeKeys(k, accountKey); err != nil {
		return nil, err
	}
	return k, nil
}

// initAccount initializes the account and prepares it for signing.
func (c *CertManager) initAccount(ctx context.Context) error {
	k, err := c.loadAccountKey()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		k, err = c.register(ctx)
		if err != nil {
			return err
		}
	}
	c.client.Key = k
	if err := c.loadCerts(); err != nil {
		return err
	}
	return nil
}
