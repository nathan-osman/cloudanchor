package container

import (
	"strconv"
	"strings"
)

const (
	labelName    = "cloudanchor.name"
	labelDomains = "cloudanchor.domains"
	labelPort    = "cloudanchor.port"
)

// Container stores the configuration for a container.
type Container struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
	Port    int      `json:"port"`
}

// New attempts to create a new container from the provided information. The
// return value is nil if required information for the container is missing.
func New(id string, labels map[string]string) *Container {

	// Name is a required label; its absence suggests that the container was
	// not intended to be used with cloudanchor
	name, ok := labels[labelName]
	if !ok {
		return nil
	}

	// The domain label is required as well; domains are comma-separated and
	// excess space should be trimmed from them
	domainStr, ok := labels[labelDomains]
	if !ok {
		return nil
	}
	domains := make([]string, 0)
	for _, d := range strings.Split(domainStr, ",") {
		domains = append(domains, strings.TrimSpace(d))
	}

	// Lastly, port is required; the reverse proxy will use this port for
	// issuing requests to the backend
	portStr, ok := labels[labelPort]
	if !ok {
		return nil
	}
	port := 0
	port, _ = strconv.Atoi(portStr)
	if port == 0 {
		return nil
	}

	return &Container{
		ID:      id,
		Name:    name,
		Domains: domains,
		Port:    port,
	}
}
