## cloudanchor

Have a bunch of web applications running in Docker? Tired of continually editing Nginx configuration files as applications are added and removed? Then cloudanchor is for you.

### Planned Features

- Monitors the Docker daemon in realtime to see when containers are started and stopped
- Uses container labels to determine the hostname to use for the server
- Creates Nginx configuration files on-demand for containers
- Integration with Let's Encrypt for TLS certificates
