package certmanager

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/acme"
)

const (
	day  = 24 * time.Hour
	week = 7 * day
)

var (
	errNoDomains    = errors.New("no domains specified")
	errNoChallenges = errors.New("unable to find a suitable challenge")
)

// performChallenge attempts to perform the specified challenge to verify a
// domain name.
func (c *CertManager) performChallenge(ctx context.Context, chal *acme.Challenge) error {
	response, err := c.client.HTTP01ChallengeResponse(chal.Token)
	if err != nil {
		return err
	}
	b := []byte(response)
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

// authorizeDomain attempts to authorize a domain name for use in a
// certificate.
func (c *CertManager) authorizeDomain(ctx context.Context, domain string) error {
	c.log.Debugf("authorizing %s...", domain)
	a, err := c.client.Authorize(ctx, domain)
	if err != nil {
		return err
	}
	if a.Status == acme.StatusValid {
		return nil
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
	return nil
}

// writeCertificates writes a certificate bundle to disk for each of the
// specified domain names and updates its state.
func (c *CertManager) writeCertificates(ders [][]byte, domains ...string) error {
	cert, err := x509.ParseCertificate(ders[0])
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	for _, b := range ders {
		err := pem.Encode(buf, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: b,
		})
		if err != nil {
			return err
		}
	}
	b := buf.Bytes()
	for _, d := range domains {
		if err := ioutil.WriteFile(c.Filename(d, TypeCert), b, 0644); err != nil {
			return err
		}
		func() {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			state := c.states[d]
			state.expires = cert.NotAfter
		}()
	}
	return nil
}

// renew attempts to create a certificate for the specified domain names and
// blocks until completion, an error, or the context is canceled.
func (c *CertManager) renew(ctx context.Context, domains ...string) error {
	c.log.Infof("renewing a certificate for %d domains", len(domains))
	if len(domains) == 0 {
		return errNoDomains
	}
	for _, d := range domains {
		if err := c.authorizeDomain(ctx, d); err != nil {
			return err
		}
	}
	k, err := c.generateKey()
	if err != nil {
		return err
	}
	csr, err := x509.CreateCertificateRequest(
		rand.Reader,
		&x509.CertificateRequest{
			Subject:  pkix.Name{CommonName: domains[0]},
			DNSNames: domains[1:],
		},
		k,
	)
	if err != nil {
		return err
	}
	ders, _, err := c.client.CreateCert(ctx, csr, 90*24*time.Hour, true)
	if err != nil {
		return err
	}
	if err := c.writeKeys(k, domains...); err != nil {
		return err
	}
	if err := c.writeCertificates(ders, domains...); err != nil {
		return err
	}
	return nil
}

// renewExpiring renews all domains that are active and near expiry. This also
// includes domains which don't have a certificate.
func (c *CertManager) renewExpiring(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	domains := []string{}
	for _, s := range c.states {
		if s.active && time.Now().Add(2*week).After(s.expires) {
			domains = append(domains, s.domain)
		}
	}
	if len(domains) == 0 {
		return nil
	}
	return c.renew(ctx, domains...)
}
