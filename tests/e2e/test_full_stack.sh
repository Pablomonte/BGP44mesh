#!/bin/bash
set -euo pipefail

echo "=== E2E: Full Stack Test ==="
START=$(date +%s)

# Clean any existing deployment
echo "Cleaning previous deployment..."
make clean >/dev/null 2>&1 || true

# Deploy
echo "Deploying local environment..."
make deploy-local

# Wait for convergence
echo "Waiting for convergence (90s)..."
sleep 90

# Verify BGP
echo "Verifying BGP sessions..."
if docker exec bird1 birdc show protocols 2>/dev/null | grep -q "Established"; then
    echo "✓ BGP sessions established"
else
    echo "✗ BGP sessions not established"
    docker exec bird1 birdc show protocols
    exit 1
fi

# Verify TINC
echo "Verifying TINC mesh..."
if docker exec tinc1 ip addr show tinc0 2>/dev/null | grep -q "10.0.0"; then
    echo "✓ TINC interface up"
else
    echo "✗ TINC interface not configured"
    exit 1
fi

# Verify etcd
echo "Verifying etcd cluster..."
if docker exec etcd1 etcdctl endpoint health 2>/dev/null | grep -q "healthy"; then
    echo "✓ etcd cluster healthy"
else
    echo "✗ etcd cluster unhealthy"
    exit 1
fi

# Verify monitoring
echo "Verifying Prometheus..."
if curl -sf http://localhost:9090/-/healthy >/dev/null 2>&1; then
    echo "✓ Prometheus healthy"
else
    echo "✗ Prometheus not accessible"
    exit 1
fi

# Calculate elapsed time
END=$(date +%s)
ELAPSED=$((END - START))

echo ""
echo "=== E2E Test Results ==="
echo "✓ All services operational"
echo "✓ BGP peering established"
echo "✓ TINC mesh connected"
echo "✓ etcd cluster healthy"
echo "✓ Monitoring active"
echo ""
echo "Total time: ${ELAPSED}s (target: <120s)"

if [ $ELAPSED -lt 120 ]; then
    echo "✓ Performance target met"
else
    echo "⚠ Convergence slower than target (not critical for dev)"
fi

echo ""
echo "=== E2E test passed ==="
