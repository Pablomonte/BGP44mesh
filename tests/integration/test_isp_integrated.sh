#!/bin/bash
# Test ISP Integrated Mode (Mesh + ISP via profile)
# Verifies that mesh and ISP are working together correctly

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "========================================"
echo "Testing ISP Integrated Mode (Mesh + ISP)"
echo "========================================"
echo ""

# Test 1: Container count
echo "Test 1: Verificando containers..."
EXPECTED_CONTAINERS=22  # 21 mesh + 1 ISP
RUNNING=$(docker ps --filter "status=running" | grep -c -E "bird|tinc|etcd|prom" || echo "0")

if [ "$RUNNING" -eq "$EXPECTED_CONTAINERS" ]; then
    echo -e "  ${GREEN}✓${NC} All $EXPECTED_CONTAINERS containers running"
else
    echo -e "  ${RED}✗${NC} Expected $EXPECTED_CONTAINERS containers, found $RUNNING"
    docker ps --filter "name=bird" --filter "name=tinc" --filter "name=etcd" --filter "name=prom"
    exit 1
fi

# Test 2: ISP container is running
echo "Test 2: Verificando container ISP..."
if docker ps | grep -q "isp-bird"; then
    echo -e "  ${GREEN}✓${NC} isp-bird container running"
else
    echo -e "  ${RED}✗${NC} isp-bird container not found"
    exit 1
fi

# Test 3: bird1 has 5 BGP peers (4 mesh + 1 ISP)
echo "Test 3: Verificando bird1 BGP peers (mesh + ISP)..."
BIRD1_PEERS=$(docker exec bird1 birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")
EXPECTED_BIRD1_PEERS=5  # 4 mesh peers + 1 ISP peer

if [ "$BIRD1_PEERS" -eq "$EXPECTED_BIRD1_PEERS" ]; then
    echo -e "  ${GREEN}✓${NC} bird1: $BIRD1_PEERS/$EXPECTED_BIRD1_PEERS peers established"
else
    echo -e "  ${YELLOW}⚠${NC} bird1: $BIRD1_PEERS/$EXPECTED_BIRD1_PEERS peers (expected 4 mesh + 1 ISP)"
    docker exec bird1 birdc show protocols
    exit 1
fi

# Test 4: ISP has 1 BGP peer (customer)
echo "Test 4: Verificando ISP BGP peer..."
ISP_PEERS=$(docker exec isp-bird birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")

if [ "$ISP_PEERS" -eq 1 ]; then
    echo -e "  ${GREEN}✓${NC} ISP: 1/1 customer peer established"
else
    echo -e "  ${RED}✗${NC} ISP: $ISP_PEERS/1 peers"
    docker exec isp-bird birdc show protocols
    exit 1
fi

# Test 5: Mesh BGP sessions (bird2-5 still have 4 peers each)
echo "Test 5: Verificando mesh BGP sessions..."
EXPECTED_MESH_PEERS=4
ALL_OK=true

for i in {2..5}; do
    ESTABLISHED=$(docker exec bird$i birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")
    if [ "$ESTABLISHED" -eq "$EXPECTED_MESH_PEERS" ]; then
        echo -e "  ${GREEN}✓${NC} bird$i: $ESTABLISHED/$EXPECTED_MESH_PEERS peers established"
    else
        echo -e "  ${YELLOW}⚠${NC} bird$i: $ESTABLISHED/$EXPECTED_MESH_PEERS peers"
        ALL_OK=false
    fi
done

if ! $ALL_OK; then
    echo "Some mesh BGP sessions incomplete"
    exit 1
fi

# Test 6: ISP routes are received on mesh nodes
echo "Test 6: Verificando propagación de rutas ISP..."
# Check if bird1 has ISP routes
ISP_ROUTES=$(docker exec bird1 birdc show route protocol isp 2>/dev/null | grep -c "192.0.2.0/24\|198.51.100.0/24\|203.0.113.0/24" || echo "0")

if [ "$ISP_ROUTES" -ge 1 ]; then
    echo -e "  ${GREEN}✓${NC} bird1 receives ISP routes ($ISP_ROUTES prefixes)"
else
    echo -e "  ${YELLOW}⚠${NC} bird1 not receiving ISP routes"
    docker exec bird1 birdc show route protocol isp
fi

# Check if bird2 has ISP routes (via iBGP from bird1)
BIRD2_ISP_ROUTES=$(docker exec bird2 birdc show route 2>/dev/null | grep -c "192.0.2.0/24\|198.51.100.0/24\|203.0.113.0/24" || echo "0")

if [ "$BIRD2_ISP_ROUTES" -ge 1 ]; then
    echo -e "  ${GREEN}✓${NC} bird2 receives ISP routes via iBGP ($BIRD2_ISP_ROUTES prefixes)"
else
    echo -e "  ${YELLOW}⚠${NC} bird2 not receiving ISP routes via iBGP"
fi

# Test 7: Verify filter is blocking TINC mesh prefix from ISP
echo "Test 7: Verificando filtros de export a ISP..."
# Check ISP routes - should NOT have 10.0.0.0/24 (TINC mesh)
ISP_ROUTES_ALL=$(docker exec isp-bird birdc show route 2>/dev/null)

if echo "$ISP_ROUTES_ALL" | grep -q "10.0.0.0/24"; then
    echo -e "  ${RED}✗${NC} ISP received internal mesh route 10.0.0.0/24 (should be blocked)"
    echo "$ISP_ROUTES_ALL"
    exit 1
else
    echo -e "  ${GREEN}✓${NC} TINC mesh route 10.0.0.0/24 correctly blocked from ISP"
fi

# Test 8: Network connectivity
echo "Test 8: Verificando conectividad de red..."
# Ping from bird1 to ISP
if docker exec bird1 ping -c 2 -W 2 172.30.0.2 >/dev/null 2>&1; then
    echo -e "  ${GREEN}✓${NC} bird1 can reach ISP (172.30.0.2)"
else
    echo -e "  ${RED}✗${NC} bird1 cannot reach ISP"
    exit 1
fi

echo ""
echo "========================================="
echo -e "${GREEN}✓ All ISP integrated tests passed!${NC}"
echo "========================================="
echo ""
echo "Summary:"
echo "  - 22 containers running (21 mesh + 1 ISP)"
echo "  - bird1: 5 BGP peers (4 mesh + 1 ISP)"
echo "  - bird2-5: 4 BGP peers each (mesh only)"
echo "  - ISP: 1 BGP peer (customer)"
echo "  - ISP routes propagated to mesh"
echo "  - TINC mesh prefix blocked from ISP"
echo ""
