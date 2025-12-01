# Technical Topics Study Guide

This document lists the key technical topics involved in the BGP4mesh hardware test. Use this guide to deepen your understanding of the networking concepts demonstrated.

---

## 📚 Key Technical Topics for Study

### 1. BGP (Border Gateway Protocol)

This is the core routing protocol used in this test. You should understand:

| Subtopic | Description |
|----------|-------------|
| **eBGP vs iBGP** | External BGP (used here between AS 65001 and AS 65000) for peering between different organizations |
| **Autonomous Systems (AS)** | AS numbers (65001 for ISP, 65000 for customer network) - private AS range |
| **BGP Sessions** | TCP port 179, session establishment, `Established` state |
| **Route Announcements** | How prefixes are advertised between peers |
| **Import/Export Filters** | Controlling which routes are accepted/announced |
| **Next-hop** | Understanding `via 172.30.0.100` - the next router to reach a destination |
| **BGP Attributes** | AS path, origin (i = IGP), preference values |

---

### 2. TINC VPN Mesh

A peer-to-peer VPN technology creating the overlay network:

| Subtopic | Description |
|----------|-------------|
| **Mesh VPN topology** | Full mesh vs hub-spoke, peer-to-peer connections |
| **Overlay vs Underlay networks** | 44.30.127.0/24 (overlay) vs 172.30.0.0/24 (underlay) |
| **TUN/TAP interfaces** | Virtual network interfaces (`tinc0`) |
| **Host files & Key exchange** | RSA public key exchange for authentication |
| **TINC protocol ports** | TCP 655 (authentication) + UDP 655 (data) |
| **Switch mode** | Layer 2 VPN operation mode |
| **ConnectTo directive** | Specifying which nodes to initiate connections to |

---

### 3. IP Routing Fundamentals

Core networking concepts demonstrated in the test:

| Subtopic | Description |
|----------|-------------|
| **Static routes** | Manual route configuration (`ip route add`) |
| **Kernel routing table** | How Linux kernel decides where to send packets |
| **Default gateway** | Route of last resort |
| **Next-hop routing** | Packet forwarding to intermediate routers |
| **IP Forwarding** | `net.ipv4.ip_forward=1` - enabling packet transit |
| **Return routes** | Why bidirectional routing is critical (reply packets must return) |
| **Longest prefix match** | How routes are selected based on specificity |

---

### 4. Docker Networking

Containerization networking concepts used throughout:

| Subtopic | Description |
|----------|-------------|
| **macvlan driver** | Assigning containers real L2 addresses on physical network |
| **Bridge networks** | Internal Docker networks (`mesh-net`, `cluster-net`) |
| **Host network mode** | Container shares host's network stack (used for ISP) |
| **Network namespaces** | Isolated network stacks per container |
| **Container networking** | `network_mode: "service:tinc1"` - sharing networks |
| **Port mapping** | Exposing container ports to host |

---

### 5. Network Architecture Concepts

High-level design patterns:

| Subtopic | Description |
|----------|-------------|
| **Border router** | Gateway between internal network and ISP |
| **ISP peering** | How customer networks connect to providers |
| **Multi-homing** | Multiple ISP connections (isp_primary + isp_secondary) |
| **Route redistribution** | Learning routes from one protocol and exporting to another |
| **Network segmentation** | Separating ISP network from mesh network |

---

### 6. Linux Network Tools & Commands

Practical tools used for verification:

| Command | Purpose |
|---------|---------|
| `ip addr show` | Display interface IP addresses |
| `ip route` | View/modify routing table |
| `ip neigh show` | View ARP table |
| `ping` / `traceroute` | Connectivity testing |
| `birdc` | BIRD routing daemon control CLI |
| `tcpdump` | Packet capture and analysis |
| `ss -tlnp` | View listening ports |
| `sysctl` | Kernel parameter configuration |

---

### 7. BIRD Internet Routing Daemon

The routing software used in the test:

| Subtopic | Description |
|----------|-------------|
| **Protocols** | Device, Kernel, Static, BGP protocol types |
| **Filters** | BIRD filter language for route manipulation |
| **Route tables** | `master4` - main IPv4 routing table |
| **Export to kernel** | Syncing BIRD routes to Linux kernel |
| **birdc CLI** | `show protocols`, `show route`, `configure` |

---

### 8. Layer 2 vs Layer 3 Concepts

| Concept | Layer | Example in Test |
|---------|-------|-----------------|
| **MAC addresses** | L2 | ARP entries, macvlan |
| **IP addresses** | L3 | 172.30.0.x, 44.30.127.x |
| **Ethernet switching** | L2 | Physical switch connecting all devices |
| **IP routing** | L3 | BGP, static routes |
| **VPN encapsulation** | L2/L3 | TINC tunnel wrapping packets |

---

### 9. Network Address Planning

IP addressing concepts:

| Concept | Example |
|---------|---------|
| **RFC 5737 Test-Net** | 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24 (blackhole routes) |
| **AMPRNet (44.x.x.x)** | 44.30.127.0/24 - amateur radio network space |
| **Private addresses** | 172.30.0.0/24 (within RFC 1918 range) |
| **CIDR notation** | /24, /32 subnet masks |
| **Host vs network routes** | 44.30.127.2/32 (host) vs 44.30.127.0/24 (network) |

---

### 10. Packet Flow Analysis

Understanding how packets traverse the network:

```
Mock-ISP (172.30.0.1)
    ↓ Kernel route: 44.30.127.0/24 via 172.30.0.100
    ↓
Laptop1 macvlan (172.30.0.100)
    ↓ IP forwarding enabled
    ↓ Route: 44.30.127.0/24 dev tinc0
    ↓
TINC tunnel (encrypted UDP)
    ↓
Laptop2 tinc0 (44.30.127.2)
    ↓
Return route: 172.30.0.1 via 44.30.127.1
    ↓ (reverse path through tunnel)
```

---

## 📖 Recommended Study Order

### Phase 1: Fundamentals
- IP addressing and subnetting
- Basic routing concepts (static routes, default gateway)
- Linux `ip` command family

### Phase 2: Intermediate
- VPN concepts (overlay/underlay)
- Docker networking basics
- BIRD routing daemon basics

### Phase 3: Advanced
- BGP (AS, eBGP peering, filters)
- TINC mesh VPN specifics
- macvlan and advanced Docker networking

---

## 🔑 Key Takeaways from the Test

From the actual test results, these are the most important lessons:

1. **macvlan is essential** for BGP on same L2 network
   - Gives container direct IP on physical network (172.30.0.100)
   - Enables BGP peering without NAT complications

2. **TINC needs both TCP+UDP** on port 655
   - TCP: Meta connections and authentication
   - UDP: Encrypted data transfer

3. **Return routes are critical** - packets must know how to get back
   - Laptop2 needs route: `172.30.0.1 via 44.30.127.1 dev tinc0`

4. **IP forwarding must be enabled** on transit routers
   - `/proc/sys/net/ipv4/ip_forward = 1`

5. **Host files need real IPs**, not container names
   - node1: `Address = 172.30.0.100` (not `tinc1`)
   - node2: `Address = 172.30.0.101` (not `tinc2`)

6. **All devices on same Ethernet switch** simplified connectivity
   - Single L2 domain (172.30.0.0/24) eliminated routing complexity

---

## 📚 Additional Resources

### BGP
- RFC 4271 - A Border Gateway Protocol 4 (BGP-4)
- BIRD User's Guide: https://bird.network.cz/?get_doc

### TINC VPN
- TINC Manual: https://www.tinc-vpn.org/documentation/

### Docker Networking
- Docker Network Drivers: https://docs.docker.com/network/

### Linux Networking
- `man ip` - Linux IP routing utilities
- Linux Advanced Routing & Traffic Control: https://lartc.org/

---

*Generated from the BGP4mesh hardware test documentation*

