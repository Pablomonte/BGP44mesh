#!/bin/bash
set -euo pipefail

echo "============================================"
echo "BIRD BGP Daemon - Entrypoint"
echo "============================================"

# Environment variables with defaults
ROUTER_ID="${ROUTER_ID:-192.0.2.1}"
BGP_AS="${BGP_AS:-65000}"

echo "Configuration:"
echo "  Router ID: $ROUTER_ID"
echo "  BGP AS: $BGP_AS"
echo ""

# Render BIRD configuration from template
if [ -f /etc/bird/bird.conf.j2 ]; then
    echo "Rendering bird.conf from template..."
    jinja2 /etc/bird/bird.conf.j2 \
        -D router_id="$ROUTER_ID" \
        -D bgp_as="$BGP_AS" \
        > /etc/bird/bird.conf

    echo "✓ Configuration rendered"
else
    echo "⚠ No template found, using static config"
fi

# Validate BIRD configuration
echo ""
echo "Validating BIRD configuration..."
if bird --parse-only -c /etc/bird/bird.conf 2>&1; then
    echo "✓ BIRD configuration valid"
else
    echo "✗ ERROR: Invalid BIRD configuration"
    echo ""
    echo "Configuration file:"
    cat /etc/bird/bird.conf
    exit 1
fi

# Display configuration files
echo ""
echo "Configuration files:"
echo "  - /etc/bird/bird.conf"
echo "  - /etc/bird/protocols.conf"
echo "  - /etc/bird/filters.conf"

# Start BIRD in foreground with debug output
echo ""
echo "Starting BIRD daemon..."
echo "============================================"

# Use exec to replace shell with bird process (PID 1)
exec bird -f -c /etc/bird/bird.conf
