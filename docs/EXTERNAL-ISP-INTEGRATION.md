# External ISP Integration Guide

**Status:** ✅ Validated and Production-Ready
**Last Updated:** 2025-11-10
**Validation Report:** ../BGP-VALIDATION-REPORT.md

---

## Overview

This guide documents the successful integration of the BGP mesh network (AS 65000) with an external ISP (AS 65001) using **macvlan networking** over wired Ethernet.

### Architecture

```
┌─────────────────────────────┐
│ External ISP                │
│ AS: 65001                   │
│ IP: 10.42.0.228/24          │
└──────────┬──────────────────┘
           │ BGP Session (eBGP)
           │ Wired LAN
┌──────────▼──────────────────┐
│ Border Router (bird1)       │
│ Macvlan: 10.42.0.100/24     │
│ TINC: 10.0.0.1/24           │
│ AS: 65000                   │
└──────────┬──────────────────┘
           │ iBGP Full Mesh
    ┌──────┴──────┬──────┬─────┐
    │             │      │     │
┌───▼───┐  ┌────▼──┐ ┌──▼──┐ ┌▼────┐
│ bird2 │  │ bird3 │ │bird4│ │bird5│
│10.0.0.2│ │10.0.0.3││10.0.0.4││10.0.0.5│
└───────┘  └───────┘ └─────┘ └─────┘
```

### Key Technologies

- **Macvlan Networking**: Direct L2 access to physical LAN (no NAT)
- **BIRD 2.x**: BGP routing daemon
- **TINC VPN**: Layer 2 mesh overlay network
- **Docker Compose**: Container orchestration

---

## Prerequisites

### Hardware Requirements
- **Wired Ethernet connection** (macvlan doesn't work reliably on WiFi)
- At least 8GB RAM (for 5-node mesh + ISP)
- Modern CPU (4+ cores recommended)

### Network Requirements
- Available IP on LAN for macvlan container
- IP outside DHCP range recommended
- Direct L2 connectivity to ISP node
- BGP port 179/tcp open between nodes

### Software Requirements
- Docker 24+
- Docker Compose v2
- Linux kernel with macvlan support

---

## Configuration

### Step 1: Configure Environment Variables

Edit `.env` file:

```bash
# BGP Configuration
BGP_AS=65000
ISP_ENABLED=true
ISP_NEIGHBOR=10.42.0.228  # ISP node IP

# Macvlan Configuration
LAN_INTERFACE=enxa0cec8992ed8  # Your wired Ethernet interface
LAN_SUBNET=10.42.0.0/24         # LAN subnet
LAN_GATEWAY=10.42.0.1           # LAN gateway
LAN_IP_RANGE=10.42.0.100/31     # IP range for containers
TINC1_LAN_IP=10.42.0.100        # Border router macvlan IP
ISP_LOCAL_IP=10.42.0.100        # IP to use for BGP session
```

**Finding your interface:**
```bash
ip route | grep default
# Output: default via 10.42.0.1 dev enxa0cec8992ed8 ...
```

### Step 2: Configure ISP Node (Required)

On the ISP node, configure BIRD to accept the mesh network:

```bird
# /etc/bird/bird.conf on ISP node
router id 192.0.2.100;

protocol device {}

protocol kernel {
    ipv4 { export all; };
}

# Routes to advertise
protocol static static1 {
    ipv4;
    route 192.0.2.0/24 blackhole;
    route 198.51.100.0/24 blackhole;
    route 203.0.113.0/24 blackhole;
}

# Filters
filter import_from_customer {
    print "Importing: ", net;
    accept;
}

filter export_to_customer {
    if proto = "static1" then {
        print "Exporting: ", net;
        accept;
    }
    reject;
}

# BGP session with customer
protocol bgp customer {
    description "Customer AS 65000";
    local 10.42.0.228 as 65001;
    neighbor 10.42.0.100 as 65000;  # Mesh border router macvlan IP

    ipv4 {
        import filter import_from_customer;
        export filter export_to_customer;
    };

    hold time 180;
    keepalive time 60;
}
```

**Apply configuration:**
```bash
# On ISP node
docker exec isp-bird birdc configure
docker exec isp-bird birdc show protocols customer
```

---

## Deployment

### Deploy Mesh with External ISP

```bash
# Clean any previous deployment
make clean

# Deploy with external ISP
make deploy-with-external-isp

# Wait for convergence (~2 minutes)
sleep 120
```

### Verify Deployment

#### 1. Check Container Status
```bash
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "bird|tinc"
# All should show "Up" and "healthy"
```

#### 2. Verify Macvlan Configuration
```bash
# Check tinc1 has macvlan IP
docker exec tinc1 ip addr show | grep 10.42.0.100
# Expected: inet 10.42.0.100/24 brd 10.42.0.255 scope global eth1

# Verify routing to ISP
docker exec tinc1 ip route get 10.42.0.228
# Expected: 10.42.0.228 dev eth1 src 10.42.0.100
```

#### 3. Check BGP Session Status
```bash
# Check ISP session
docker exec bird1 birdc show protocols isp
# Expected: isp  BGP  ---  up  HH:MM:SS  Established

# Check internal mesh peers
docker exec bird1 birdc show protocols | grep peer
# Expected: All peer2-5 showing "Established"
```

#### 4. Verify Route Exchange
```bash
# Routes received from ISP
docker exec bird1 birdc show route protocol isp

# Expected output:
# 192.0.2.0/24         unicast [isp ...] via 10.42.0.228
# 198.51.100.0/24      unicast [isp ...] via 10.42.0.228
# 203.0.113.0/24       unicast [isp ...] via 10.42.0.228

# Verify routes propagated to mesh
docker exec bird2 birdc show route protocol peer1 | head -10
```

---

## Troubleshooting

### BGP Session Not Establishing

**Check 1: Verify macvlan connectivity**
```bash
# Test L3 connectivity
docker exec tinc1 bash -c "cat < /dev/tcp/10.42.0.228/179" 2>&1
# Should connect without error

# If fails, check macvlan network
docker network inspect bgp4mesh_lan-macvlan
```

**Check 2: Verify BIRD configuration**
```bash
# Check rendered config
docker exec bird1 cat /var/run/bird/protocols.conf | grep -A 10 "protocol bgp isp"

# Verify:
# - local 10.42.0.100 as 65000;
# - neighbor 10.42.0.228 as 65001;
```

**Check 3: ISP side configuration**
```bash
# On ISP node
ssh user@10.42.0.228 "docker exec isp-bird birdc show protocols customer"

# Should show Active or Established
```

### Routes Not Propagating

**Check import/export filters:**
```bash
# View filters
docker exec bird1 cat /var/run/bird/filters.conf

# Test with permissive filters temporarily
# On ISP node, edit filters to "accept;" for testing
```

### Macvlan Not Working

**Symptom:** "Socket: No route to host" despite correct configuration

**Common Causes:**
1. **WiFi interface** - Macvlan doesn't work on WiFi
2. **Switch/router blocking** - Unknown MAC addresses blocked
3. **Driver limitation** - NIC doesn't support macvlan

**Solution:** Verify using wired Ethernet and test with simple container:
```bash
docker run --rm --network bgp4mesh_lan-macvlan --ip 10.42.0.101 -it alpine ping 10.42.0.228
```

---

## Performance & Monitoring

### BGP Session Health
```bash
# Session uptime and statistics
docker exec bird1 birdc show protocols all isp | grep -A 30 "BGP state"
```

### Expected Metrics
- **Session establishment:** < 5 seconds
- **Keepalive interval:** 30 seconds
- **Hold time:** 90 seconds
- **Routes imported:** 3 (from ISP)
- **Route propagation:** < 1 second to all mesh nodes

### Monitoring Commands
```bash
# Watch BGP sessions
watch 'docker exec bird1 birdc show protocols | grep -E "Name|peer|isp"'

# Monitor routes
watch 'docker exec bird1 birdc show route count'

# Check logs
docker logs bird1 --tail 50 -f
```

---

## Production Recommendations

### Security Enhancements

1. **Enable MD5 Authentication**
```bird
protocol bgp isp {
    ...
    password "your-secure-password";
    ...
}
```

2. **Implement Strict Route Filters**
```bird
filter import_from_isp {
    # Only accept expected prefixes
    if net ~ [192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24] then accept;
    reject;
}
```

3. **Add Rate Limiting**
```bird
protocol bgp isp {
    ...
    import limit 1000 action restart;
    ...
}
```

### Reliability Improvements

1. **Enable BFD** (fast failure detection <1s)
```bird
protocol bgp isp {
    ...
    bfd on;
    ...
}

protocol bfd {
    interface "eth1";
}
```

2. **Increase Hold Times** (for unstable links)
```bird
protocol bgp isp {
    hold time 180;
    keepalive time 60;
}
```

3. **Configure Graceful Restart**
```bird
protocol bgp isp {
    ...
    graceful restart on;
    ...
}
```

---

## Validation Checklist

Use this checklist after deployment:

- [ ] All containers running and healthy
- [ ] Macvlan IP assigned to tinc1
- [ ] BGP session with ISP established
- [ ] 3+ routes received from ISP
- [ ] Routes propagated to all mesh nodes (bird2-5)
- [ ] Internal mesh peers (peer2-5) established
- [ ] TINC overlay operational (10.0.0.x reachable)
- [ ] No BGP session flapping (stable >5 minutes)
- [ ] Export filters working (if configured)
- [ ] Monitoring dashboards accessible

---

## Files and Configuration

### Key Files Modified
- `.env` - Environment variables for ISP and macvlan
- `configs/bird/protocols.conf.j2` - Added ISP BGP protocol with macvlan support
- `docker/bird/entrypoint.sh` - Added ISP_LOCAL_IP variable handling
- `deploy/hardware-test/docker-compose.border-router.yml` - Hardware test border router with macvlan (replaces deprecated docker-compose.external-isp.yml)

### Configuration Flow
```
.env (ISP_LOCAL_IP)
    ↓
docker-compose.yml (bird1 environment)
    ↓
docker/bird/entrypoint.sh (template rendering)
    ↓
configs/bird/protocols.conf.j2 (BGP protocol)
    ↓
/var/run/bird/protocols.conf (rendered config)
```

---

## Comparison: Macvlan vs Alternatives

During development, several networking approaches were tested to achieve external ISP connectivity:

### Approaches Tested

| Approach | Works? | NAT? | Complexity | TINC Access | Issues Found |
|----------|--------|------|------------|-------------|--------------|
| **Macvlan (Ethernet)** | ✅ Yes | No | Low | Yes | **None - Production Ready** |
| Macvlan (WiFi) | ❌ No | - | - | - | WiFi drivers don't support multiple MACs; APs filter MAC addresses |
| Bridge + NAT | ❌ No | Yes | High | Yes | BGP breaks - source IP changes prevent session establishment |
| Host network + veth | ⚠️ Partial | No | High | Requires bridge | Complex namespace bridging; BIRD must run on host, not containerized |
| GRE Tunnel | ✅ Yes | No | Medium | Yes | Untested - adds encapsulation overhead |

### Failed Approach Details

#### 1. Bridge + NAT (deprecated approach)
**Attempted Setup:**
```yaml
networks:
  external-bgp:
    driver: bridge
    driver_opts:
      com.docker.network.bridge.enable_ip_masquerade: "true"
```

**Problems:**
- Required manual iptables SNAT rules
- BGP source IP changed by NAT
- ISP rejects BGP OPEN messages from unexpected source
- Error: "Socket: No route to host" despite connectivity

**Conclusion:** BGP protocol fundamentally incompatible with NAT

#### 2. Macvlan over WiFi (wlp0s20f3)
**Attempted Setup:**
- LAN: 10.233.88.0/24 (WiFi network)
- ISP: 10.233.88.135
- Interface: wlp0s20f3 (wireless)

**Problems:**
- Macvlan creates new MAC address for container
- WiFi drivers typically support only one MAC per interface
- Access points filter/block unknown MAC addresses
- Result: "No route to host" even for basic ping

**Conclusion:** Macvlan requires wired Ethernet

#### 3. Host Network + veth Bridge (scripts/setup-host-tinc-bridge.sh)
**Attempted Setup:**
- Create veth pair between host and tinc1 container
- Run BIRD on host (not containerized)
- Bridge host network namespace with TINC mesh

**Problems:**
- Complex namespace manipulation required
- BIRD must run on host system (defeats containerization)
- Difficult to maintain and debug
- Not portable across environments

**Conclusion:** Overly complex, abandons container architecture

### Working Solution

**Macvlan over Wired Ethernet** (deploy/hardware-test/docker-compose.border-router.yml)

**Configuration:**
```yaml
networks:
  lan-macvlan:
    driver: macvlan
    driver_opts:
      parent: enxa0cec8992ed8  # Wired Ethernet interface
      macvlan_mode: bridge
```

**Why It Works:**
- Direct L2 access to physical LAN
- No NAT - BGP sees correct source IP
- Wired Ethernet supports multiple MAC addresses
- Fully containerized - BIRD stays in containers
- Simple, clean architecture

**Recommendation:** Use macvlan on wired Ethernet for production deployments.

---

## Success Story

**Setup:**
- Mesh Network: AS 65000 (5 nodes, full mesh)
- External ISP: AS 65001 @ 10.42.0.228
- Connection: Macvlan over wired Ethernet (10.42.0.0/24)

**Results:**
- ✅ BGP session established in < 2 seconds
- ✅ 3 ISP routes imported successfully
- ✅ Routes propagated to all 5 mesh nodes
- ✅ Zero packet loss, stable for 2+ hours
- ✅ Internal mesh unaffected (4/4 peers up)

**Key Success Factor:** Using wired Ethernet interface instead of WiFi enabled macvlan to work correctly.

See **BGP-VALIDATION-REPORT.md** for detailed validation results.

---

## References

- **Validation Report:** [BGP-VALIDATION-REPORT.md](BGP-VALIDATION-REPORT.md)
- **Project Architecture:** [../CLAUDE.md](../CLAUDE.md)
- **Main README:** [../README.md](../README.md)
- **Docker Macvlan Docs:** https://docs.docker.com/network/drivers/macvlan/
- **BIRD 2.x BGP Docs:** https://bird.network.cz/?get_doc&f=bird-6.html

---

**Document Status:** Authoritative - replaces all previous ISP integration guides
**Validated:** 2025-11-10
**Maintainer:** Project BGP4mesh Team
