package certmanager

import (
	"fmt"
	"path"
	"strings"
)

const (
	typeKey  = "key"
	typeCert = "crt"
)

// filename determines the filename to use for the specified type of item.
func (c *CertManager) filename(domain, type_ string) string {
	return path.Join(
		c.cfg.Directory,
		fmt.Sprintf(
			"%s.%s",
			strings.Replace(domain, ".", "_", -1),
			type_,
		),
	)
}
