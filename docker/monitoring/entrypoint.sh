#!/bin/bash
set -euo pipefail

echo "============================================"
echo "Monitoring Stack - Entrypoint"
echo "============================================"
echo "Starting Prometheus + Grafana"
echo ""

# Create prometheus data directory
mkdir -p /prometheus
chown -R nobody:nobody /prometheus

# Use custom prometheus config if provided
if [ -f /etc/prometheus/prometheus.yml ]; then
    echo "✓ Using custom Prometheus configuration"
    PROM_CONFIG="/etc/prometheus/prometheus.yml"
else
    echo "⚠ Using default Prometheus configuration"
    PROM_CONFIG="/etc/prometheus/prometheus.yml.default"
fi

# Start Prometheus in background
echo ""
echo "Starting Prometheus on port 9090..."
prometheus \
    --config.file="$PROM_CONFIG" \
    --storage.tsdb.path=/prometheus \
    --web.console.libraries=/usr/share/prometheus/console_libraries \
    --web.console.templates=/usr/share/prometheus/consoles \
    --web.listen-address=0.0.0.0:9090 \
    &

PROM_PID=$!
echo "✓ Prometheus started (PID: $PROM_PID)"

# Wait a bit for Prometheus to start
sleep 5

# Start Grafana in foreground
echo ""
echo "Starting Grafana on port 3000..."
echo "============================================"

# Set Grafana defaults
export GF_PATHS_PROVISIONING=/etc/grafana/provisioning
export GF_SECURITY_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"
export GF_USERS_ALLOW_SIGN_UP=false

cd /usr/share/grafana

# Trap signals to cleanup
trap "kill $PROM_PID" SIGTERM SIGINT

# Start Grafana
exec /run.sh
