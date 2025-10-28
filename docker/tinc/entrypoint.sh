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

echo "Configuration:"
echo "  Node name: $TINC_NAME"
echo "  Node ID: $NODE_ID"
echo "  Port: $TINC_PORT"
echo "  Network: $TINC_NETNAME"
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

# Always create host file (regenerate on each start)
if [ -f "$TINC_DIR/rsa_key.pub" ]; then
    echo "Creating host file..."
    # Use Docker service name for address resolution (tinc1, tinc2, tinc3)
    CONTAINER_NAME="tinc$NODE_ID"

    cat > "$TINC_DIR/hosts/$TINC_NAME" << EOF
# Host configuration for $TINC_NAME
Address = $CONTAINER_NAME
Port = $TINC_PORT

EOF
    cat "$TINC_DIR/rsa_key.pub" >> "$TINC_DIR/hosts/$TINC_NAME"
    echo "✓ Host file created (Address = $CONTAINER_NAME)"
fi

# Render TINC configuration from template
if [ -f /etc/tinc/tinc.conf.j2 ]; then
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

    # Add ConnectTo directives for full mesh (Sprint 1: hardcoded for 3 nodes)
    echo "Adding ConnectTo directives for mesh..."
    for i in 1 2 3; do
        if [ "node$i" != "$TINC_NAME" ]; then
            echo "ConnectTo = node$i" >> "$TINC_DIR/tinc.conf"
        fi
    done
    echo "✓ ConnectTo directives added"
fi

# Render tinc-up script
if [ -f /etc/tinc/tinc-up.j2 ]; then
    echo "Rendering tinc-up script..."
    python3 << EOF
from jinja2 import Template

with open('/etc/tinc/tinc-up.j2', 'r') as f:
    template = Template(f.read())

output = template.render(tinc_name='$TINC_NAME', node_id='$NODE_ID')

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

# Use exec to replace shell with tincd process (PID 1)
# -D = no daemon, stay in foreground
# -d3 = debug level 3
# Using full config path
exec tincd -c /var/run/tinc/"$TINC_NETNAME" -D -d3
