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

    # Add bootstrap ConnectTo directives
    # Strategy: All nodes connect to node1 for initial star topology
    # Once daemon syncs host files, TINC will discover full mesh
    echo ""
    echo "Adding bootstrap ConnectTo directives..."

    if [ "$TINC_NAME" != "node1" ]; then
        # All nodes except node1 connect to node1
        echo "ConnectTo = node1" >> "$TINC_DIR/tinc.conf"
        echo "  - node1 (bootstrap hub)"
    else
        # node1 also connects to others for redundancy
        echo "ConnectTo = node2" >> "$TINC_DIR/tinc.conf"
        echo "ConnectTo = node3" >> "$TINC_DIR/tinc.conf"
        echo "  - node2, node3 (bootstrap redundancy)"
    fi

    echo "✓ Bootstrap topology configured"

    # Wait for daemons to sync host files from etcd
    # Give other nodes time to register and daemons time to propagate
    echo ""
    echo "Waiting for host file propagation (10s)..."
    sleep 10

    # Check if we have host files for peers we're trying to connect to
    MISSING_HOSTS=""
    if [ "$TINC_NAME" != "node1" ]; then
        [ ! -f "$TINC_DIR/hosts/node1" ] && MISSING_HOSTS="node1 $MISSING_HOSTS"
    else
        [ ! -f "$TINC_DIR/hosts/node2" ] && MISSING_HOSTS="node2 $MISSING_HOSTS"
        [ ! -f "$TINC_DIR/hosts/node3" ] && MISSING_HOSTS="node3 $MISSING_HOSTS"
    fi

    if [ -n "$MISSING_HOSTS" ]; then
        echo "⚠ Warning: Missing host files for: $MISSING_HOSTS"
        echo "   TINC may not connect until daemon syncs these files"
    else
        echo "✓ All required host files present"
    fi
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

# Start tincd in foreground mode
# -D = no daemon, stay in foreground (required for Docker)
# -d3 = debug level 3
# --logfile = write logs to file for debugging
# Using full config path
exec tincd -c /var/run/tinc/"$TINC_NETNAME" -D -d3 --logfile="/var/run/tinc/$TINC_NETNAME/tinc.log"
