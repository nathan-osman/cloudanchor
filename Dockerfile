FROM nginx:latest
MAINTAINER Nathan Osman <nathan@quickmediasolutions.com>

# Add the root CAs
ADD https://curl.haxx.se/ca/cacert.pem /etc/ssl/certs/

# Install supervisord
RUN \
    apt-get update && \
    apt-get install -y supervisor && \
    rm -rf /var/lib/apt/lists/*

# Copy the supervisord configuration
ADD supervisord.conf /etc/supervisor/supervisord.conf

# Add the cloudanchor binary
ADD dist/cloudanchor /usr/local/bin/cloudanchor

# Run supervisord
CMD ["/usr/bin/supervisord"]
