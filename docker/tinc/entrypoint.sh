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

    # Add placeholder for ConnectTo (daemon will manage them)
    echo "# ConnectTo directives will be managed by bgp-daemon" >> "$TINC_DIR/tinc.conf"
    echo "✓ Placeholder added (daemon will manage ConnectTo)"
elif [ -f "$TINC_DIR/tinc.conf" ]; then
    echo "✓ Using existing tinc.conf (managed by daemon)"
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

# Start tincd in daemon mode (creates pidfile for reload support)
# -d3 = debug level 3
# Using full config path
# Note: Don't use -D (foreground) so pidfile is created for cross-container reload
tincd -c /var/run/tinc/"$TINC_NETNAME" -d3

# Keep container running and tail logs
echo "TINC daemon started, monitoring logs..."
tail -f /var/run/tinc/"$TINC_NETNAME"/tinc.log 2>/dev/null || sleep infinity
