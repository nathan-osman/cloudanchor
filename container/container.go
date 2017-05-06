package container

import (
	"strings"

	"github.com/docker/docker/api/types"
)

const (
	labelAddr    = "cloudanchor.addr"
	labelDomains = "cloudanchor.domains"
)

// Container stores the configuration for a container.
type Container struct {
	ID      string
	Name    string
	Domains []string
	Addr    string
}

// New attempts to create a new container from the provided information. The
// return value is nil if required information for the container is missing.
func New(cJSON types.ContainerJSON) *Container {

	// The domain label is required as well; domains are comma-separated and
	// excess space should be trimmed from them
	domainStr, ok := cJSON.Config.Labels[labelDomains]
	if !ok {
		return nil
	}
	domains := make([]string, 0)
	for _, d := range strings.Split(domainStr, ",") {
		domains = append(domains, strings.TrimSpace(d))
	}

	// Address is required for setting up the reverse proxy
	addr, ok := cJSON.Config.Labels[labelAddr]
	if !ok {
		return nil
	}

	// Create the container
	return &Container{
		ID:      cJSON.ID,
		Name:    cJSON.Name,
		Domains: domains,
		Addr:    addr,
	}
}
