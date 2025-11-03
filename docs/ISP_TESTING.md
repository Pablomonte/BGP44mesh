# ISP Testing Guide

## Overview

This document describes how to test BGP connectivity with a simulated ISP upstream. The mock ISP allows testing realistic eBGP scenarios, route filtering, and failover without requiring external infrastructure.

**Mock ISP Specifications:**
- **AS Number**: 65001 (simulated ISP)
- **IP Address**: 172.30.0.2 (on isp-net)
- **Announces**: TEST-NET prefixes (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24)
- **Accepts**: Customer prefixes (10.100.0.0/24, 10.200.0.0/24)
- **Blocks**: Internal TINC mesh (10.0.0.0/24)

**Border Router (bird1):**
- **IP on ISP network**: 172.30.0.1
- **Role**: Gateway between mesh (AS 65000) and ISP (AS 65001)
- **BGP Sessions**: 4 iBGP (mesh) + 1 eBGP (ISP)

## Deployment Modes

### Mode 1: Mesh Only (Default - Sprint 1.5)

**Use Case**: Standard mesh testing without upstream ISP

```bash
# Deploy
make deploy-local

# Verify
docker ps  # Should show 21 containers
docker exec bird1 birdc show protocols  # Should show 4/4 peers

# Characteristics
- 21 containers: 5 bird + 5 tinc + 5 daemon + 5 etcd + 1 prometheus
- bird1-5: Each has 4 BGP peers (full mesh iBGP)
- No ISP connectivity
- ISP_ENABLED defaults to false
```

**When to Use:**
- Default development and testing
- TINC mesh testing
- iBGP full mesh testing
- Pre-ISP development

---

### Mode 2: Integrated (Mesh + ISP via Profile)

**Use Case**: Testing mesh with upstream ISP on the same host

```bash
# Deploy
make deploy-local-isp
# Or manually:
ISP_ENABLED=true docker compose --profile isp up -d --build

# Verify
docker ps  # Should show 22 containers (21 mesh + 1 ISP)
docker exec bird1 birdc show protocols  # Should show 5/5 peers (4 mesh + 1 ISP)
docker exec isp-bird birdc show protocols  # Should show 1/1 peer (customer)

# Test
make test-isp-integrated

# Characteristics
- 22 containers: 21 mesh + 1 isp-bird
- bird1: 5 BGP peers (4 iBGP mesh + 1 eBGP ISP)
- bird2-5: 4 BGP peers each (iBGP mesh only)
- ISP routes propagated to all mesh nodes via iBGP
- Route filtering active (10.0.0.0/24 blocked from ISP)
```

**When to Use:**
- Testing eBGP connectivity
- Route filtering validation
- ISP route propagation to mesh
- Single-host integration testing

**Verification Commands:**

```bash
# Check ISP BGP session on bird1
docker exec bird1 birdc show protocols isp

# Check ISP routes received
docker exec bird1 birdc show route protocol isp

# Verify ISP routes propagated to bird2 (via iBGP)
docker exec bird2 birdc show route | grep "192.0.2.0/24"

# Check what ISP sees (should NOT have 10.0.0.0/24)
docker exec isp-bird birdc show route

# Verify connectivity
docker exec bird1 ping -c 3 172.30.0.2  # Ping ISP
```

---

### Mode 3: Decoupled (Hybrid - Separate Hosts)

**Use Case**: Testing with ISP running on a different host/network

#### Scenario A: ISP on Host A, Mesh on Host B

**Host A (ISP):**
```bash
cd /path/to/BGP
make deploy-isp-only

# Verify ISP is listening
docker exec isp-bird birdc show status
docker inspect isp-bird | grep IPAddress  # Note the IP

# Make ISP accessible from external hosts
# Option 1: Port forward BGP (if using different networks)
# Option 2: Use Docker bridge network routing
```

**Host B (Mesh):**
```bash
# Set ISP external IP
export ISP_NEIGHBOR=<host-a-ip>  # e.g., 192.168.1.100
export ISP_ENABLED=true

# Deploy mesh
docker compose up -d --build

# Verify bird1 connects to external ISP
docker exec bird1 birdc show protocols isp
docker exec bird1 ping -c 3 $ISP_NEIGHBOR
```

#### Scenario B: Simulating WAN Link Latency

```bash
# On mesh host, add latency to ISP link
docker exec bird1 tc qdisc add dev eth0 root netem delay 50ms

# Test BGP convergence time
docker exec bird1 birdc show protocols all isp | grep "Last error"

# Remove latency
docker exec bird1 tc qdisc del dev eth0 root
```

**When to Use:**
- Testing with realistic WAN separation
- Multi-host lab environments
- Simulating network latency/issues
- ISP failover testing

---

## Route Filtering

### Mesh to ISP (Export)

**Policy**: Only announce customer prefixes

```conf
# In configs/bird/filters.conf
filter export_to_isp {
    # Accept customer prefixes
    if net ~ [10.100.0.0/24, 10.200.0.0/24] then accept;

    # Reject TINC mesh internal network
    if net ~ [10.0.0.0/24] then reject;

    # Reject everything else
    reject;
}
```

**Rationale:**
- `10.100.0.0/24`, `10.200.0.0/24`: Customer networks (should be routed via Internet)
- `10.0.0.0/24`: Internal TINC mesh (private, should NOT leak to ISP)

**Verification:**
```bash
# ISP should see customer prefixes
docker exec isp-bird birdc show route | grep "10.100.0.0/24"  # Should appear
docker exec isp-bird birdc show route | grep "10.200.0.0/24"  # Should appear

# ISP should NOT see mesh prefix
docker exec isp-bird birdc show route | grep "10.0.0.0/24"  # Should NOT appear
```

### ISP to Mesh (Import)

**Policy**: Accept all ISP routes with high local-pref

```conf
filter import_from_isp {
    bgp_local_pref = 200;  # Prefer ISP routes
    accept;
}
```

**Rationale:**
- Accept all legitimate Internet routes from ISP
- High local-pref (200) ensures ISP routes are preferred over any internal default

**Verification:**
```bash
# Check ISP routes on bird1
docker exec bird1 birdc show route protocol isp

# Verify local-pref
docker exec bird1 birdc show route all 192.0.2.0/24 | grep "BGP.local_pref"
# Should show: BGP.local_pref: 200

# Check propagation to bird2 via iBGP
docker exec bird2 birdc show route 192.0.2.0/24
```

---

## Testing Procedures

### Test 1: Mesh-Only Backward Compatibility

**Purpose**: Verify ISP changes don't break existing mesh

```bash
# Clean environment
make clean-all

# Deploy mesh only (no ISP)
make deploy-local

# Verify (should be identical to Sprint 1.5)
docker ps | wc -l  # Should be 21 containers
docker exec bird1 birdc show protocols | grep -c Established  # Should be 4

# Run standard tests
make test-integration
```

**Expected Result**: ✓ All tests pass, identical to pre-ISP behavior

### Test 2: ISP Integrated Mode

**Purpose**: Verify ISP + mesh integration

```bash
# Clean environment
make clean-all

# Deploy with ISP
make deploy-local-isp

# Wait for convergence (~30s)
sleep 30

# Run ISP tests
make test-isp-integrated
```

**Expected Results:**
- ✓ 22 containers running
- ✓ bird1: 5 BGP sessions (4 mesh + 1 ISP)
- ✓ ISP routes received on all mesh nodes
- ✓ Customer routes announced to ISP
- ✓ TINC mesh prefix blocked from ISP

### Test 3: ISP Failover

**Purpose**: Verify mesh continues working if ISP fails

```bash
# Deploy with ISP
make deploy-local-isp

# Verify ISP is up
docker exec bird1 birdc show protocols isp | grep Established

# Stop ISP
docker stop isp-bird

# Wait 90s (BGP hold timer)
sleep 90

# Verify mesh still works
for i in {1..5}; do
  docker exec bird$i birdc show protocols | grep -c Established
done
# bird1 should show 4/4 (mesh only)
# bird2-5 should show 4/4 (unchanged)

# Restart ISP
docker start isp-bird

# Verify reconvergence (~30s)
sleep 30
docker exec bird1 birdc show protocols isp | grep Established
```

**Expected Result**: ✓ Mesh unaffected by ISP failure, ISP reconnects automatically

---

## Troubleshooting

### ISP Container Not Starting

```bash
# Check logs
docker logs isp-bird

# Common issues:
# 1. Port 179 conflict
netstat -tuln | grep 179
# Solution: Change port in docker-compose.isp.yml

# 2. Network conflict
docker network inspect bgp-isp-net
# Solution: make clean-all && make deploy-local-isp

# 3. Config syntax error
docker exec isp-bird bird -p -c /etc/bird/bird.conf
```

### bird1 Not Connecting to ISP

```bash
# Check ISP_ENABLED
docker exec bird1 env | grep ISP_ENABLED
# Should be: ISP_ENABLED=true

# Check rendered config
docker exec bird1 cat /var/run/bird/protocols.conf | grep -A 10 "protocol bgp isp"
# Should show ISP peer config

# Check connectivity
docker exec bird1 ping -c 3 172.30.0.2
# If fails: Network issue

# Check BIRD logs
docker logs bird1 | grep -i "isp\|172.30.0.2"

# Manual BGP troubleshooting
docker exec bird1 birdc show protocols all isp
```

### ISP Routes Not Propagating to Mesh

```bash
# Check if bird1 receives routes from ISP
docker exec bird1 birdc show route protocol isp
# Should show 3 routes (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24)

# Check if bird1 exports routes to mesh peers
docker exec bird1 birdc show route export peer1

# Check if bird2 imports routes from bird1
docker exec bird2 birdc show route protocol peer1

# Check iBGP session
docker exec bird2 birdc show protocols all peer1 | grep "BGP state"
```

### TINC Mesh Prefix Leaking to ISP

```bash
# This is a CRITICAL security issue - internal network exposed to ISP!

# Check ISP routes
docker exec isp-bird birdc show route | grep "10.0.0.0/24"
# Should be EMPTY

# If present, check filter
docker exec bird1 cat /etc/bird/filters.conf | grep -A 10 "export_to_isp"

# Verify filter is applied
docker exec bird1 birdc show protocols all isp | grep "Export filter"
# Should show: Export filter: export_to_isp

# Test filter manually
docker exec bird1 birdc eval "filter export_to_isp" "10.0.0.0/24"
# Should reject
```

---

## Performance Benchmarks

### Expected Convergence Times

| Scenario | Time |
|----------|------|
| Initial mesh startup (no ISP) | ~90s |
| Initial mesh + ISP startup | ~120s |
| ISP peer added to running mesh | ~30s |
| ISP failure detection | ~90s (hold timer) |
| ISP reconnection | ~10s |

### Resource Usage

| Mode | Containers | RAM | CPU (idle) |
|------|-----------|-----|------------|
| Mesh only | 21 | ~8GB | ~5% |
| Mesh + ISP | 22 | ~8.2GB | ~5% |
| ISP only | 1 | ~50MB | ~0.1% |

---

## Advanced Scenarios

### Scenario: Multiple ISPs (Future)

```yaml
# docker-compose.yml (conceptual)
services:
  isp-bird-1:
    profiles: ["isp"]
    networks:
      isp-net:
        ipv4_address: 172.30.0.2

  isp-bird-2:
    profiles: ["isp"]
    networks:
      isp-net:
        ipv4_address: 172.30.0.3
```

### Scenario: ISP with BGP Communities

```conf
# configs/isp-bird/bird.conf (future enhancement)
protocol bgp customer {
    ipv4 {
        export filter {
            bgp_community.add((65001,100));  # Tag ISP routes
            accept;
        };
    };
}
```

---

## Cleanup

```bash
# Clean mesh only
make clean

# Clean ISP only
make clean-isp

# Clean everything (mesh + ISP + networks)
make clean-all
```

---

## Summary

| Mode | Containers | Command | Use Case |
|------|-----------|---------|----------|
| **Mesh Only** | 21 | `make deploy-local` | Default development |
| **Integrated** | 22 | `make deploy-local-isp` | Single-host ISP testing |
| **Decoupled** | 1 ISP + 21 mesh | `make deploy-isp-only` (separate hosts) | Multi-host lab |

**Key Takeaways:**
- ISP is opt-in via profile (backward compatible)
- Only bird1 connects to ISP (border router)
- Route filtering prevents TINC mesh leakage
- All 3 modes can coexist for different test scenarios
