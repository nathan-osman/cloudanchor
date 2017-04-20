package certmanager

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	TypeKey  = "key"
	TypeCert = "crt"
)

var errInvalidFilename = errors.New("invalid filename")

var reDomain = regexp.MustCompile(`([^/]+)\.\w+$`)

// Filename determines the filename to use for the specified type of item.
func (c *CertManager) Filename(domain, type_ string) string {
	return path.Join(
		c.cfg.Directory,
		fmt.Sprintf(
			"%s.%s",
			strings.Replace(domain, ".", "_", -1),
			type_,
		),
	)
}

// domain attempts to determine the domain name, given a filename.
func (c *CertManager) domain(filename string) (string, error) {
	m := reDomain.FindStringSubmatch(path.Base(filename))
	if len(m) == 0 {
		return "", errInvalidFilename
	}
	return strings.Replace(m[1], "_", ".", -1), nil
}
