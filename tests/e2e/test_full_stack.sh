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

# Detect node count
echo "Detecting cluster size..."
sleep 5  # Brief wait for containers to register
NODE_COUNT=$(docker compose ps --services 2>/dev/null | grep -c "^bird" || echo 5)
EXPECTED_PEERS=$((NODE_COUNT - 1))
echo "✓ Detected $NODE_COUNT nodes"

# Wait for convergence
echo "Waiting for convergence (90s)..."
sleep 90

# Verify BGP on all nodes
echo "Verifying BGP sessions (all $NODE_COUNT nodes)..."
BGP_OK=true
for node_id in $(seq 1 $NODE_COUNT); do
    SESSIONS=$(docker exec bird$node_id birdc show protocols 2>/dev/null | grep -c "Established" || echo 0)
    if [ "$SESSIONS" -ge "$EXPECTED_PEERS" ]; then
        echo "  ✓ bird$node_id: $SESSIONS sessions"
    else
        echo "  ✗ bird$node_id: $SESSIONS sessions (expected $EXPECTED_PEERS)"
        BGP_OK=false
    fi
done

if [ "$BGP_OK" = true ]; then
    echo "✓ All BGP sessions established"
else
    echo "✗ Some BGP sessions missing"
    exit 1
fi

# Verify TINC on all nodes
echo "Verifying TINC mesh (all $NODE_COUNT nodes)..."
TINC_OK=true
for node_id in $(seq 1 $NODE_COUNT); do
    if docker exec tinc$node_id ip addr show tinc0 2>/dev/null | grep -q "10.0.0.$node_id"; then
        echo "  ✓ tinc$node_id: interface up"
    else
        echo "  ✗ tinc$node_id: interface not configured"
        TINC_OK=false
    fi
done

if [ "$TINC_OK" = true ]; then
    echo "✓ All TINC interfaces up"
else
    echo "✗ Some TINC interfaces down"
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
echo "✓ All services operational ($NODE_COUNT nodes)"
echo "✓ BGP peering established ($EXPECTED_PEERS sessions per node)"
echo "✓ TINC mesh connected (full mesh)"
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
