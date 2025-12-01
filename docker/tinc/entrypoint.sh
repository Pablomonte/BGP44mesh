#!/bin/bash
set -euo pipefail

echo "============================================"
echo "TINC VPN Mesh - Entrypoint"
echo "============================================"

# Environment variables with defaults
TINC_NAME="${TINC_NAME:-node1}"
TINC_PORT="${TINC_PORT:-655}"
TINC_NETNAME="${TINC_NETNAME:-bgpmesh}"

# Extract node number from name (node1 → 1)
NODE_ID="${TINC_NAME: -1}"

# Host file configuration (can be overridden via environment)
# TINC_ADDRESS: reachable IP/hostname for this node (default: container name for docker-compose local testing)
# TINC_SUBNET: subnet this node announces (default: 44.30.127.x/32 based on node ID)
TINC_ADDRESS="${TINC_ADDRESS:-tinc$NODE_ID}"
TINC_SUBNET="${TINC_SUBNET:-44.30.127.$NODE_ID/32}"

echo "Configuration:"
echo "  Node name: $TINC_NAME"
echo "  Node ID: $NODE_ID"
echo "  Port: $TINC_PORT"
echo "  Network: $TINC_NETNAME"
echo "  Address: $TINC_ADDRESS"
echo "  Subnet: $TINC_SUBNET"
echo ""

# Create TINC directory structure in writable location
TINC_DIR="/var/run/tinc/$TINC_NETNAME"
mkdir -p "$TINC_DIR/hosts"

# Generate RSA keys if they don't exist
if [ ! -f "$TINC_DIR/rsa_key.priv" ]; then
    echo "Generating RSA 2048-bit keys..."
    cd "$TINC_DIR"
    # Generate keys using correct config path
    echo -e "\n\n" | tincd -c "$TINC_DIR" -K2048
    echo "✓ RSA keys generated"
else
    echo "✓ Using existing RSA keys"
fi

# Create host file only if it doesn't exist (preserve manual fixes on restart)
# The host file contains the public key and network configuration for this node
if [ -f "$TINC_DIR/rsa_key.pub" ]; then
    if [ ! -f "$TINC_DIR/hosts/$TINC_NAME" ]; then
        echo "Creating host file..."
        cat > "$TINC_DIR/hosts/$TINC_NAME" << EOF
# Host configuration for $TINC_NAME
Address = $TINC_ADDRESS
Port = $TINC_PORT
Subnet = $TINC_SUBNET

EOF
        cat "$TINC_DIR/rsa_key.pub" >> "$TINC_DIR/hosts/$TINC_NAME"
        echo "✓ Host file created (Address = $TINC_ADDRESS, Subnet = $TINC_SUBNET)"
    else
        echo "✓ Using existing host file (preserved across restarts)"
        echo "  Current configuration:"
        grep -E "^(Address|Subnet)" "$TINC_DIR/hosts/$TINC_NAME" | sed 's/^/    /'
    fi
fi

# Render TINC configuration from template (only if it doesn't exist)
if [ -f /etc/tinc/tinc.conf.j2 ] && [ ! -f "$TINC_DIR/tinc.conf" ]; then
    echo ""
    echo "Rendering tinc.conf from template..."
    python3 << EOF
from jinja2 import Template

with open('/etc/tinc/tinc.conf.j2', 'r') as f:
    template = Template(f.read())

output = template.render(tinc_name='$TINC_NAME', tinc_port='$TINC_PORT')

with open('$TINC_DIR/tinc.conf', 'w') as f:
    f.write(output)
EOF
    echo "✓ tinc.conf rendered"

    # No bootstrap ConnectTo directives - daemon will manage connections dynamically
    echo ""
    echo "⚠ No initial ConnectTo directives"
    echo "   Daemon will manage connections dynamically via CLI"
    echo "✓ Bootstrap configuration ready"
elif [ -f "$TINC_DIR/tinc.conf" ]; then
    echo "✓ Using existing tinc.conf"
fi

# Render tinc-up script
if [ -f /etc/tinc/tinc-up.j2 ]; then
    echo "Rendering tinc-up script..."
    python3 << EOF
from jinja2 import Template

with open('/etc/tinc/tinc-up.j2', 'r') as f:
    template = Template(f.read())

output = template.render(tinc_name='$TINC_NAME', node_id='$NODE_ID', hostname='$(hostname)')

with open('$TINC_DIR/tinc-up', 'w') as f:
    f.write(output)
EOF
    chmod +x "$TINC_DIR/tinc-up"
    echo "✓ tinc-up rendered and executable"
fi

# Render tinc-down script
if [ -f /etc/tinc/tinc-down.j2 ]; then
    echo "Rendering tinc-down script..."
    python3 << EOF
from jinja2 import Template

with open('/etc/tinc/tinc-down.j2', 'r') as f:
    template = Template(f.read())

output = template.render(tinc_name='$TINC_NAME')

with open('$TINC_DIR/tinc-down', 'w') as f:
    f.write(output)
EOF
    chmod +x "$TINC_DIR/tinc-down"
    echo "✓ tinc-down rendered and executable"
fi

# Display configuration
echo ""
echo "Configuration files:"
echo "  - $TINC_DIR/tinc.conf"
echo "  - $TINC_DIR/tinc-up"
echo "  - $TINC_DIR/tinc-down"
echo "  - $TINC_DIR/rsa_key.priv"

# Start TINC daemon
echo ""
echo "Starting TINC daemon..."
echo "============================================"

# Start tincd in foreground mode
# -D = no daemon, stay in foreground (required for Docker)
# -d3 = debug level 3
# --logfile = write logs to file for debugging
# Using full config path
exec tincd -c /var/run/tinc/"$TINC_NETNAME" -D -d3 --logfile="/var/run/tinc/$TINC_NETNAME/tinc.log"
