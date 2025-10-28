#!/bin/bash
set -euo pipefail

echo "=== Testing Docker Builds ==="

# Build images in parallel
echo "Building bird image..."
docker build ./docker/bird -t bgp-bird:test &
BIRD_PID=$!

echo "Building tinc image..."
docker build ./docker/tinc -t bgp-tinc:test &
TINC_PID=$!

echo "Building monitoring image..."
docker build ./docker/monitoring -t bgp-monitoring:test &
MONITORING_PID=$!

# Wait for all builds
wait $BIRD_PID && echo "✓ BIRD image built" || { echo "✗ BIRD build failed"; exit 1; }
wait $TINC_PID && echo "✓ TINC image built" || { echo "✗ TINC build failed"; exit 1; }
wait $MONITORING_PID && echo "✓ Monitoring image built" || { echo "✗ Monitoring build failed"; exit 1; }

echo "=== All Docker images built successfully ==="
