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

var (
	errNoDomains    = errors.New("no domains specified")
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

// authorizeDomain attempts to authorize a domain name for use in a
// certificate.
func (c *CertManager) authorizeDomain(ctx *context.Context, domain string) error {
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
	return nil
}

// writeCertificates writes a certificate bundle to disk for each of the
// specified domain names.
func (c *CertManager) writeCertificates(ders [][]byte, domains ...string) error {
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
		if err := ioutil.WriteFile(c.filename(d, typeCert), b, 0644); err != nil {
			return err
		}
	}
	return nil
}

// renew attempts to create a certificate for the specified domain names and
// blocks until completion, an error, or the context is canceled.
func (c *CertManager) renew(ctx *context.Context, domains ...string) error {
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
