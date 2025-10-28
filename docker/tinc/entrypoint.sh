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

# Create TINC directory structure
TINC_DIR="/etc/tinc/$TINC_NETNAME"
mkdir -p "$TINC_DIR/hosts"

# Generate RSA keys if they don't exist
if [ ! -f "$TINC_DIR/rsa_key.priv" ]; then
    echo "Generating RSA 2048-bit keys..."
    tinc -n "$TINC_NETNAME" generate-keys 2048 <<EOF


EOF
    echo "✓ RSA keys generated"
else
    echo "✓ Using existing RSA keys"
fi

# Render TINC configuration from template
if [ -f /etc/tinc/tinc.conf.j2 ]; then
    echo ""
    echo "Rendering tinc.conf from template..."
    jinja2 /etc/tinc/tinc.conf.j2 \
        -D tinc_name="$TINC_NAME" \
        -D tinc_port="$TINC_PORT" \
        > "$TINC_DIR/tinc.conf"

    echo "✓ tinc.conf rendered"
fi

# Render tinc-up script
if [ -f /etc/tinc/tinc-up.j2 ]; then
    echo "Rendering tinc-up script..."
    jinja2 /etc/tinc/tinc-up.j2 \
        -D tinc_name="$TINC_NAME" \
        -D node_id="$NODE_ID" \
        > "$TINC_DIR/tinc-up"
    chmod +x "$TINC_DIR/tinc-up"
    echo "✓ tinc-up rendered and executable"
fi

# Render tinc-down script
if [ -f /etc/tinc/tinc-down.j2 ]; then
    echo "Rendering tinc-down script..."
    jinja2 /etc/tinc/tinc-down.j2 \
        -D tinc_name="$TINC_NAME" \
        > "$TINC_DIR/tinc-down"
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
exec tincd -n "$TINC_NETNAME" -D -d3
