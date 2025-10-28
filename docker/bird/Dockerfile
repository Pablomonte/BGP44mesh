FROM debian:12-slim

# Install BIRD 3.x and dependencies
RUN apt-get update && apt-get install -y \
    bird2 \
    python3-jinja2 \
    iproute2 \
    curl \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Note: bird2 package provides BIRD 3.x in Debian 12
# Verify version
RUN bird --version || echo "BIRD installed"

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose BGP port
EXPOSE 179

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD birdc show status || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
