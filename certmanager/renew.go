package certmanager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"golang.org/x/crypto/acme"
)

var (
	errNoChallenges = errors.New("unable to find a suitable challenge")
)

// performChallenge attempts to perform the specified challenge to verify a
// domain name.
func (c *CertManager) performChallenge(ctx *context.Context, chal *acme.Challenge) error {
	b := []byte(c.client.HTTP01ChallengeResponse(chal.Token))
	mux := http.NewServeMux()
	mux.HandleFunc(
		c.client.HTTP01ChallengePath(chal.Token),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			w.WriteHeader(http.StatusOK)
			w.Write(b)
		},
	)
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", c.cfg.Port))
	if err != nil {
		return nil
	}
	defer l.Close()
	go func() {
		http.Serve(l, mux)
	}()
	a, err := c.client.Accept(ctx, chal)
	if err != nil {
		return err
	}
	_, err = c.client.WaitAuthorization(ctx, a.URI)
	return err
}

// renew attempts to create a certificate for the specified domain names and
// blocks until completion, an error, or the context is canceled.
func (c *CertManager) renew(ctx *context.Context, domains ...string) error {

	// Create a new key for the domains
	k, err := generateKey()
	if err != nil {
		return err
	}

	// Each of the domains needs to be authorized
	for _, d := range domains {
		a, err := c.client.Authorize(ctx, domain)
		if err != nil {
			return err
		}
		if a.Status == acme.StatusValid {
			continue
		}
		var chal *acme.Challenge
		for _, c := range a.Challenges {
			if c.Type == "http-01" {
				chal = c
			}
		}
		if chal == nil {
			return errNoChallenges
		}
		if err := c.performChallenge(ctx, chal); err != nil {
			return err
		}
	}

	//...
}
