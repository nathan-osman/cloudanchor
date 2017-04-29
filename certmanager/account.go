package certmanager

import (
	"context"
	"os"

	"golang.org/x/crypto/acme"
)

const accountKey = "account"

// register prepares the account for use by generating a new private key. The
// account URI is written to a JSON file.
func (c *CertManager) register(ctx context.Context) error {
	c.log.Debug("generating account key...")
	k, err := c.generateKey()
	if err != nil {
		return err
	}
	c.client.Key = k
	if err := c.writeKeys(k, accountKey); err != nil {
		return err
	}
	_, err = c.client.Register(ctx, nil, acme.AcceptTOS)
	if err != nil {
		return err
	}
	return nil
}

// initAccount initializes the account and prepares it for signing.
func (c *CertManager) initAccount(ctx context.Context) error {
	c.log.Debug("initializing account...")
	k, err := c.loadKey(accountKey)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		c.log.Debug("account key does not exist")
		if err = c.register(ctx); err != nil {
			return err
		}
	} else {
		c.client.Key = k
	}
	if err := c.loadCerts(); err != nil {
		return err
	}
	return nil
}
