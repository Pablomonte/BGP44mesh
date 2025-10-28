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

# Create writable config directory
mkdir -p /var/run/bird

# Render BIRD configuration from template
if [ -f /etc/bird/bird.conf.j2 ]; then
    echo "Rendering bird.conf from template..."
    python3 << EOF
from jinja2 import Template
import sys

with open('/etc/bird/bird.conf.j2', 'r') as f:
    template = Template(f.read())

output = template.render(router_id='$ROUTER_ID', bgp_as='$BGP_AS')

with open('/var/run/bird/bird.conf', 'w') as f:
    f.write(output)
EOF

    # Copy static includes to writable location
    cp /etc/bird/protocols.conf /var/run/bird/ 2>/dev/null || true
    cp /etc/bird/filters.conf /var/run/bird/ 2>/dev/null || true

    # Update include paths in rendered config
    sed -i 's|/etc/bird/|/var/run/bird/|g' /var/run/bird/bird.conf

    echo "✓ Configuration rendered"
else
    echo "⚠ No template found, using static config"
    # Copy static config if exists
    if [ -f /etc/bird/bird.conf ]; then
        cp /etc/bird/* /var/run/bird/
    fi
fi

# Validate BIRD configuration (BIRD 2.x doesn't have --parse-only)
echo ""
echo "Skipping config validation (BIRD 2.x limitation)"
echo "Configuration will be validated on startup"

# Display configuration files
echo ""
echo "Configuration files:"
echo "  - /var/run/bird/bird.conf"
echo "  - /var/run/bird/protocols.conf"
echo "  - /var/run/bird/filters.conf"

# Start BIRD in foreground with debug output
echo ""
echo "Starting BIRD daemon..."
echo "============================================"

# Use exec to replace shell with bird process (PID 1)
exec bird -f -c /var/run/bird/bird.conf
