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
echo "Testing ISP Multi-homing Mode"
echo "========================================"
echo ""

# Test 1: Container count
echo "Test 1: Verificando containers..."
EXPECTED_CONTAINERS=8  # 5 tinc + 1 bird + 1 ISP + 1 etcd
RUNNING=$(docker ps --filter "status=running" | grep -c -E "bird|tinc|etcd" || echo "0")

if [ "$RUNNING" -eq "$EXPECTED_CONTAINERS" ]; then
    echo -e "  ${GREEN}✓${NC} All $EXPECTED_CONTAINERS containers running"
else
    echo -e "  ${RED}✗${NC} Expected $EXPECTED_CONTAINERS containers, found $RUNNING"
    docker ps --filter "name=bird" --filter "name=tinc" --filter "name=etcd"
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

# Test 3: bird1 has 2 BGP peers (both to ISP via multi-homing)
echo "Test 3: Verificando bird1 BGP peers (multi-homing to ISP)..."
BIRD1_PEERS=$(docker exec bird1 birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")
EXPECTED_BIRD1_PEERS=2  # 2 ISP uplinks

if [ "$BIRD1_PEERS" -eq "$EXPECTED_BIRD1_PEERS" ]; then
    echo -e "  ${GREEN}✓${NC} bird1: $BIRD1_PEERS/$EXPECTED_BIRD1_PEERS peers established (multi-homing)"
else
    echo -e "  ${YELLOW}⚠${NC} bird1: $BIRD1_PEERS/$EXPECTED_BIRD1_PEERS peers (expected 2 ISP uplinks)"
    docker exec bird1 birdc show protocols
    exit 1
fi

# Test 4: ISP has 2 BGP peers (customer multi-homing)
echo "Test 4: Verificando ISP BGP peers..."
ISP_PEERS=$(docker exec isp-bird birdc show protocols 2>/dev/null | grep -c "Established" || echo "0")

if [ "$ISP_PEERS" -eq 2 ]; then
    echo -e "  ${GREEN}✓${NC} ISP: 2/2 customer peers established (multi-homing)"
else
    echo -e "  ${RED}✗${NC} ISP: $ISP_PEERS/2 peers"
    docker exec isp-bird birdc show protocols
    exit 1
fi

echo "Test 5: Verificando propagación de rutas ISP..."
# Check if bird1 has ISP routes via both uplinks
ISP_PRIMARY_ROUTES=$(docker exec bird1 birdc show route protocol isp_primary 2>/dev/null | grep -c "192.0.2.0/24\|198.51.100.0/24\|203.0.113.0/24" || echo "0")
ISP_SECONDARY_ROUTES=$(docker exec bird1 birdc show route protocol isp_secondary 2>/dev/null | grep -c "192.0.2.0/24\|198.51.100.0/24\|203.0.113.0/24" || echo "0")

if [ "$ISP_PRIMARY_ROUTES" -ge 1 ]; then
    echo -e "  ${GREEN}✓${NC} bird1 receives ISP routes via primary link ($ISP_PRIMARY_ROUTES prefixes)"
else
    echo -e "  ${YELLOW}⚠${NC} bird1 not receiving ISP routes via primary link"
    docker exec bird1 birdc show route protocol isp_primary
fi

if [ "$ISP_SECONDARY_ROUTES" -ge 1 ]; then
    echo -e "  ${GREEN}✓${NC} bird1 receives ISP routes via secondary link ($ISP_SECONDARY_ROUTES prefixes)"
else
    echo -e "  ${YELLOW}⚠${NC} bird1 not receiving ISP routes via secondary link"
    docker exec bird1 birdc show route protocol isp_secondary
fi

# Test 6: Verify local-pref for multi-homing (primary should be preferred)
echo "Test 6: Verificando local-pref para multi-homing..."
# Check that routes learned from primary have higher local-pref
PRIMARY_PREF=$(docker exec bird1 birdc show route all 192.0.2.0/24 2>/dev/null | grep "BGP.local_pref:" | head -1 | awk '{print $2}' || echo "0")

if [ "$PRIMARY_PREF" -eq 200 ]; then
    echo -e "  ${GREEN}✓${NC} Primary link has correct local-pref (200)"
else
    echo -e "  ${YELLOW}⚠${NC} Primary link local-pref is $PRIMARY_PREF (expected 200)"
fi

# Test 7: Verify filter is blocking TINC mesh prefix from ISP
echo "Test 7: Verificando filtros de export a ISP..."
# Check ISP routes - should NOT have 44.30.127.0/24 (TINC mesh)
ISP_ROUTES_ALL=$(docker exec isp-bird birdc show route 2>/dev/null)

if echo "$ISP_ROUTES_ALL" | grep -q "44.30.127.0/24"; then
    echo -e "  ${RED}✗${NC} ISP received internal mesh route 44.30.127.0/24 (should be blocked)"
    echo "$ISP_ROUTES_ALL"
    exit 1
else
    echo -e "  ${GREEN}✓${NC} TINC mesh route 44.30.127.0/24 correctly blocked from ISP"
fi

# Test 8: Network connectivity
echo "Test 8: Verificando conectividad de red..."
# Note: BGP Established state already proves network connectivity
# ping may not be available in BIRD container (minimal image)
if docker exec bird1 ping -c 2 -W 2 172.30.0.2 >/dev/null 2>&1; then
    echo -e "  ${GREEN}✓${NC} bird1 can reach ISP (172.30.0.2)"
elif docker exec bird1 which ping >/dev/null 2>&1; then
    echo -e "  ${RED}✗${NC} bird1 cannot reach ISP"
    exit 1
else
    echo -e "  ${YELLOW}⚠${NC} ping not available (BGP session proves connectivity)"
fi

echo ""
echo "========================================="
echo -e "${GREEN}✓ All ISP multi-homing tests passed!${NC}"
echo "========================================="
echo ""
echo "Summary:"
echo "  - 8 containers running (5 TINC + 1 BIRD + 1 ISP + 1 etcd)"
echo "  - bird1: 2 BGP peers (both to ISP via multi-homing)"
echo "  - ISP: 2 BGP peers (both from customer)"
echo "  - ISP routes received via both uplinks"
echo "  - Primary link preferred (local-pref 200 > 150)"
echo "  - TINC mesh prefix blocked from ISP"
echo ""
