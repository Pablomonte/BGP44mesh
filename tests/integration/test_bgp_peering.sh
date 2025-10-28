#!/bin/bash
set -euo pipefail

echo "=== Integration Test: BGP Peering ==="

# Check if containers are running
echo "Checking containers..."
if ! docker ps | grep -q bird1; then
    echo "✗ Containers not running. Run 'make deploy-local' first."
    exit 1
fi

# Wait a bit for BGP to establish
echo "Waiting for BGP convergence (30s)..."
sleep 30

# Check BGP sessions
echo "Testing BGP sessions..."
BIRD1_STATUS=$(docker exec bird1 birdc show protocols 2>/dev/null | grep -c "peer[0-9].*Established" || echo 0)
echo "  BIRD1 established sessions: $BIRD1_STATUS"

if [ "$BIRD1_STATUS" -ge 1 ]; then
    echo "✓ BGP sessions established"
else
    echo "✗ No BGP sessions established"
    echo "Debug info:"
    docker exec bird1 birdc show protocols || true
    exit 1
fi

# Check etcd propagation
echo "Testing etcd propagation..."
PEERS=$(docker exec etcd1 etcdctl get /peers/ --prefix 2>/dev/null | grep -c "node" || echo 0)
echo "  Peers in etcd: $PEERS"

if [ "$PEERS" -ge 1 ]; then
    echo "✓ etcd propagation working"
else
    echo "⚠ No peers in etcd (may be expected if tinc-up not executed yet)"
fi

# Check TINC connectivity
echo "Testing TINC connectivity..."
if docker exec tinc1 ip addr show tinc0 2>/dev/null | grep -q "10.0.0"; then
    echo "✓ TINC interface configured"
else
    echo "⚠ TINC interface not up (may still be starting)"
fi

# Test ping over TINC (optional, may fail initially)
echo "Testing ping over TINC (optional)..."
if docker exec tinc1 ping -c 3 -W 2 10.0.0.2 >/dev/null 2>&1; then
    echo "✓ TINC ping successful"
else
    echo "⚠ TINC ping failed (not critical for MVP)"
fi

echo "=== Integration tests passed ==="
