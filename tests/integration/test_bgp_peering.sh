#!/bin/bash
set -euo pipefail

echo "=== Integration Test: BGP Peering ==="

# Detect node count dynamically from running containers
echo "Detecting cluster size..."
NODE_COUNT=$(docker compose ps --services 2>/dev/null | grep -c "^bird" || echo 0)
if [ "$NODE_COUNT" -eq 0 ]; then
    # Fallback: count running bird containers
    NODE_COUNT=$(docker ps --format '{{.Names}}' | grep -c "^bird" || echo 0)
fi

if [ "$NODE_COUNT" -lt 2 ]; then
    echo "✗ Insufficient nodes detected (found $NODE_COUNT, need ≥2). Run 'make deploy-local' first."
    exit 1
fi

EXPECTED_PEERS=$((NODE_COUNT - 1))  # Full mesh: N-1 peers per node
echo "✓ Detected $NODE_COUNT nodes (expecting $EXPECTED_PEERS peers per node)"

# Wait a bit for BGP to establish
echo "Waiting for BGP convergence (30s)..."
sleep 30

# Check BGP sessions on ALL nodes
echo "Testing BGP sessions (full mesh validation)..."
ALL_SESSIONS_OK=true
for node_id in $(seq 1 $NODE_COUNT); do
    SESSIONS=$(docker exec bird$node_id birdc show protocols 2>/dev/null | grep -c "peer.*Established" 2>/dev/null || echo "0")
    SESSIONS=$(echo "$SESSIONS" | head -n 1 | tr -d '[:space:]')

    if [ "$SESSIONS" -ge "$EXPECTED_PEERS" ] 2>/dev/null; then
        echo "  ✓ bird$node_id: $SESSIONS/$EXPECTED_PEERS sessions established"
    else
        echo "  ✗ bird$node_id: $SESSIONS/$EXPECTED_PEERS sessions (INSUFFICIENT)"
        echo "Debug info for bird$node_id:"
        docker exec bird$node_id birdc show protocols || true
        ALL_SESSIONS_OK=false
    fi
done

if [ "$ALL_SESSIONS_OK" = true ]; then
    echo "✓ All BGP sessions established (full mesh verified)"
else
    echo "✗ Some BGP sessions missing (see details above)"
    exit 1
fi

# Check etcd propagation
echo "Testing etcd propagation..."
PEERS=$(docker exec etcd1 etcdctl get /peers/ --prefix 2>/dev/null | grep -c "node" || echo 0)
echo "  Peers in etcd: $PEERS"

if [ "$PEERS" -ge "$NODE_COUNT" ]; then
    echo "✓ etcd propagation working ($PEERS/$NODE_COUNT peers registered)"
else
    echo "✗ Insufficient peers in etcd (expected $NODE_COUNT, got $PEERS)"
    docker exec etcd1 etcdctl get /peers/ --prefix || true
    exit 1
fi

# Check TINC interface
echo "Testing TINC interface..."
if docker exec tinc1 ip addr show tinc0 2>/dev/null | grep -q "10.0.0"; then
    echo "✓ TINC interface configured"
else
    echo "✗ TINC interface not up"
    exit 1
fi

# Check daemon synced host files (each daemon should have N host files: self + N-1 peers)
echo "Testing daemon host file sync..."
HOST_FILES=$(docker exec daemon1 ls /var/run/tinc/bgpmesh/hosts/ 2>/dev/null | wc -l)
echo "  Host files synced: $HOST_FILES"
if [ "$HOST_FILES" -ge "$NODE_COUNT" ]; then
    echo "✓ Daemon synced host files ($HOST_FILES files, includes self + peers)"
else
    echo "✗ Insufficient host files (expected $NODE_COUNT, got $HOST_FILES)"
    docker exec daemon1 ls -la /var/run/tinc/bgpmesh/hosts/ || true
    exit 1
fi

# Check TINC connections configured via daemon (TINC 1.0 file-based)
echo "Testing TINC dynamic connections..."
TINC_CONNS=$(docker exec tinc1 grep -c "^ConnectTo" /var/run/tinc/bgpmesh/tinc.conf 2>/dev/null || echo 0)
echo "  Configured ConnectTo directives: $TINC_CONNS"
if [ "$TINC_CONNS" -ge "$EXPECTED_PEERS" ]; then
    echo "✓ TINC full mesh configured ($TINC_CONNS/$EXPECTED_PEERS peers)"
else
    echo "✗ Insufficient ConnectTo directives (expected $EXPECTED_PEERS, got $TINC_CONNS)"
    echo "Debug: tinc.conf content:"
    docker exec tinc1 cat /var/run/tinc/bgpmesh/tinc.conf || true
    echo "Debug: Daemon logs (last 30 lines):"
    docker logs --tail 30 daemon1 || true
    exit 1
fi

# Test ping over TINC - Full mesh validation (all pairs)
# Note: Using daemon containers for ping (they share network namespace with tinc)
echo "Testing TINC connectivity (full mesh ping)..."
PING_FAILURES=0
TOTAL_PINGS=$((NODE_COUNT * EXPECTED_PEERS))
SUCCESSFUL_PINGS=0

for src_id in $(seq 1 $NODE_COUNT); do
    for dst_id in $(seq 1 $NODE_COUNT); do
        if [ "$src_id" != "$dst_id" ]; then
            if docker exec daemon$src_id ping -c 1 -W 2 10.0.0.$dst_id >/dev/null 2>&1; then
                SUCCESSFUL_PINGS=$((SUCCESSFUL_PINGS + 1))
            else
                echo "  ✗ Ping failed: node$src_id → 10.0.0.$dst_id"
                PING_FAILURES=$((PING_FAILURES + 1))
            fi
        fi
    done
done

if [ "$PING_FAILURES" -eq 0 ]; then
    echo "✓ Full mesh TINC connectivity verified ($SUCCESSFUL_PINGS/$TOTAL_PINGS pings successful)"
else
    echo "✗ Some pings failed ($PING_FAILURES failures out of $TOTAL_PINGS)"
    echo "Debug: tinc.conf content:"
    docker exec tinc1 cat /var/run/tinc/bgpmesh/tinc.conf || true
    echo "Debug: TINC daemon logs:"
    docker logs --tail 20 tinc1 || true
    exit 1
fi

echo ""
echo "=== Integration tests passed ==="
echo "Summary: $NODE_COUNT nodes, full mesh validated"
echo "  - BGP: $EXPECTED_PEERS sessions per node ✓"
echo "  - etcd: $NODE_COUNT peers registered ✓"
echo "  - TINC: $EXPECTED_PEERS connections per node ✓"
echo "  - Connectivity: $TOTAL_PINGS pings successful ✓"
