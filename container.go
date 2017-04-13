package cloudanchor

// Container stores the configuration for a container.
type Container struct {
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
	Host    string   `json:"host"`
	Port    int      `json:"port"`
}
