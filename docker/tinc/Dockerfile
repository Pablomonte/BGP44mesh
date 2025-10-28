FROM debian:12-slim

# Install TINC 1.0 and dependencies
RUN apt-get update && apt-get install -y \
    tinc \
    iproute2 \
    python3-jinja2 \
    curl \
    procps \
    etcd-client \
    && rm -rf /var/lib/apt/lists/*

# Pin TINC 1.0 to prevent upgrades
RUN apt-mark hold tinc

# Verify TINC version
RUN tincd --version || echo "TINC installed"

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose TINC UDP port
EXPOSE 655/udp

# Health check - verify tinc0 interface exists
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD ip addr show tinc0 || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
