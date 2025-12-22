#!/bin/bash
set -euo pipefail

MAX_WAIT="${MAX_WAIT:-60}"

echo "=== BIRD Border Router ==="
echo "AS: ${BORDER_ROUTER_AS:-unknown}, Router ID: ${BORDER_ROUTER_IP:-unknown}"
echo "Peering with ISP at ${ISP_IP:-unknown} (AS ${ISP_AS:-unknown})"

# Find Netmaker WireGuard interface (nm-* or netmaker)
find_interface() {
    ip -o link show | awk -F': ' '{print $2}' | grep -E '^(nm-|netmaker)' | head -1
}

echo "Waiting for Netmaker WireGuard interface..."

waited=0
while true; do
    INTERFACE=$(find_interface)
    if [ -n "$INTERFACE" ]; then
        break
    fi
    if [ $waited -ge $MAX_WAIT ]; then
        echo "ERROR: No nm-* interface found after ${MAX_WAIT}s"
        ip link show
        exit 1
    fi
    sleep 2
    waited=$((waited + 2))
    echo "  waiting... (${waited}s)"
done

echo "Found interface: $INTERFACE"
ip addr show "$INTERFACE" | grep -E "inet |link/"

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

# Prepare BIRD runtime directory
mkdir -p /run/bird
chown bird:bird /run/bird

echo "Starting BIRD..."
exec bird -f -c /etc/bird/bird.conf
