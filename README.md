## cloudanchor

Have a bunch of web applications running in Docker? Tired of continually editing Nginx or Apache configuration files as applications are added and removed? Then cloudanchor is for you.

> **Warning:** this application is still a work in progress. Although the features listed below are known to work, this app has not been thoroughly tested in production yet. As such, its use in production is discouraged.

### Features

- Monitors the Docker daemon in realtime to see when containers are started and stopped
- Uses container labels to determine the hostname to use for the server
- Creates configuration files for your web server on-demand
- Integration with Let's Encrypt for TLS certificates
