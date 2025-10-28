#!/bin/bash
set -euo pipefail

echo "=== Validating .env.example ==="

# Check if .env.example exists
if [ ! -f .env.example ]; then
    echo "✗ .env.example not found"
    exit 1
fi

# Check for required variables
REQUIRED_VARS=("BGP_AS" "TINC_PORT" "ETCD_INITIAL_CLUSTER" "BIRD_PASSWORD" "TINC_NETNAME" "GRAFANA_ADMIN_PASSWORD")

for var in "${REQUIRED_VARS[@]}"; do
    if grep -q "^${var}=" .env.example; then
        echo "✓ $var found"
    else
        echo "✗ $var missing"
        exit 1
    fi
done

echo "=== All required environment variables present ==="
