package container

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
)

const (
	labelDomains = "cloudanchor.domains"
	labelPort    = "cloudanchor.port"
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

	// Port is also required; the reverse proxy will use this port for issuing
	// requests to the backend
	portStr, ok := cJSON.Config.Labels[labelPort]
	if !ok {
		return nil
	}
	port := 0
	port, _ = strconv.Atoi(portStr)
	if port == 0 {
		return nil
	}

	// Create the container
	return &Container{
		ID:      cJSON.ID,
		Name:    cJSON.Name,
		Domains: domains,
		Addr:    fmt.Sprintf("%s:%d", cJSON.NetworkSettings.IPAddress, port),
	}
}
