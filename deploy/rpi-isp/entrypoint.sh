#!/bin/bash
set -euo pipefail

echo "=== BIRD BGP Daemon (Mock ISP) ==="
echo "Router ID: ${ISP_IP:-172.30.0.1}"
echo "BGP AS: ${ISP_AS:-65001}"
echo "Peering with Border Router at ${BORDER_ROUTER_IP:-172.30.0.100} (AS ${BORDER_ROUTER_AS:-65000})"

# Generate BIRD configuration from template
echo "Generating BIRD configuration from template..."
envsubst < /etc/bird/bird.conf.template > /etc/bird/bird.conf

# Verify generated configuration
echo "Verifying BIRD configuration..."
bird -p -c /etc/bird/bird.conf || {
    echo "ERROR: Invalid BIRD configuration generated!"
    echo "--- Generated config ---"
    cat /etc/bird/bird.conf
    exit 1
}

mkdir -p /run/bird
chown bird:bird /run/bird

# Start BIRD in foreground
echo "Starting BIRD..."
exec bird -f -c /etc/bird/bird.conf
