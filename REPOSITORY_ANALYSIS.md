# BGP4mesh Repository - Complete In-Depth Analysis

## Table of Contents

1. [Project Overview](#project-overview)
2. [Technology Stack Explained](#technology-stack-explained)
3. [Architecture & How Everything Works](#architecture--how-everything-works)
4. [Component Deep Dive](#component-deep-dive)
5. [File Structure Explained](#file-structure-explained)
6. [How to Use This Project](#how-to-use-this-project)
7. [Development Workflow](#development-workflow)
8. [Key Concepts for Beginners](#key-concepts-for-beginners)
9. [Testing Infrastructure](#testing-infrastructure)
10. [Future Roadmap](#future-roadmap)

---

## Project Overview

### What is This Project?

**BGP4mesh** is a production-grade networking system that creates a **BGP (Border Gateway Protocol) overlay network** over a **TINC mesh VPN**. 

In simple terms:
- It allows multiple computers (nodes) to communicate securely through encrypted tunnels (TINC VPN)
- These nodes automatically discover each other and exchange routing information (BGP)
- The system is self-organizing, fault-tolerant, and scalable
- Everything is automated through Docker containers and custom software

### The Problem It Solves

Imagine you have 5 servers in different locations and you want them to:
1. **Communicate securely** - encrypted connections
2. **Know about each other automatically** - no manual configuration for every new server
3. **Route traffic intelligently** - if one server goes down, traffic automatically reroutes
4. **Scale easily** - adding a new server is as simple as running a command

This project solves all these problems by combining several powerful networking technologies.

### Current Status

- **Sprint 1**: ✅ Completed - Basic 3-node mesh with Docker
- **Sprint 2 Phase 1**: ✅ Completed (Oct 2025)
  - 5-node deployment
  - 92.7% test coverage for core components
  - Full Ansible automation
  - Prometheus/Grafana monitoring
- **Sprint 2 Phase 2**: 🚧 In Progress - Enhanced dashboards, additional tests
- **Sprint 3**: 📅 Planned - Production hardening
- **Sprint 4**: 📅 Planned - Advanced features (RPKI, route reflectors)

---

## Technology Stack Explained

Let me explain each technology used and *why* it was chosen:

### 1. **BIRD (BGP Routing Daemon) - Version 3.x**

**What it is:**
- A routing daemon that implements the BGP protocol
- BGP is the protocol that powers the entire Internet - it's how routers tell each other about available networks

**What it does here:**
- Runs on each node
- Establishes BGP sessions with other nodes over the TINC mesh
- Exchanges routing information automatically
- Updates the Linux kernel routing table

**Why BIRD 3.x specifically?**
- Modern MP-BGP support (handles both IPv4 and IPv6 in one daemon)
- RPKI validation for security (validates route origins)
- BFD integration for fast failure detection (<30 seconds)
- Lower memory footprint (~100MB) compared to alternatives like FRR (~200MB)
- Active development and security updates

**Configuration:**
- Config file: `bird.conf` (main settings)
- Protocol definitions: `protocols.conf` (BGP peers)
- Filters: `filters.conf` (route policies)

---

### 2. **TINC VPN - Version 1.0**

**What it is:**
- A VPN (Virtual Private Network) that creates encrypted tunnels between nodes
- Operates in "switch mode" - behaves like a Layer 2 network switch

**What it does here:**
- Creates encrypted connections between all nodes (mesh topology)
- Every node can talk directly to every other node
- Handles NAT traversal (works even if nodes are behind firewalls)
- Provides a virtual network interface (`tinc0`) with private IP addresses (10.0.0.0/24)

**Why TINC 1.0 specifically?**
- **Switch mode**: Full Layer 2 mesh, transparent to BGP
- **Legacy compatibility**: Works on OpenWrt routers (important for future production deployment)
- **Battle-tested**: Stable and reliable
- **NAT traversal**: UDP hole punching works behind firewalls
- **RSA-2048 encryption**: Strong security with upgrade path to RSA-4096

**Trade-offs:**
- Manual key exchange (automated by the Go daemon)
- Slightly higher latency than WireGuard (~50ms overhead vs ~20ms)
- Older codebase, but stability is more important for this use case

**Configuration:**
- Main config: `tinc.conf` (mode, port, connections)
- Host files: One per node with public key and IP
- Scripts: `tinc-up` (run when VPN starts), `tinc-down` (run when stops)

---

### 3. **etcd - Version 3.5.14+**

**What it is:**
- A distributed key-value database
- Uses the Raft consensus algorithm for consistency

**What it does here:**
- Stores information about all peers in the network
- Each node registers itself: `/peers/node1`, `/peers/node2`, etc.
- Provides real-time notifications when peers join or leave (watch API)
- Ensures all nodes have a consistent view of the network

**Why etcd?**
- **Lightweight**: Only 50MB per node (vs 200MB for Kafka, 500MB+ for Consul)
- **Raft consensus**: Strong consistency, tolerates failures (3-node quorum can lose 1 node)
- **Watch API**: Real-time updates for the Go daemon
- **Low latency**: <10ms reads for peer lookups
- **Simple operations**: No complex dependencies like Zookeeper

**Data stored:**
```
/peers/node1 → {IP: 10.0.0.1, Key: <RSA_PUBLIC_KEY>, Endpoint: tinc1:655}
/peers/node2 → {IP: 10.0.0.2, Key: <RSA_PUBLIC_KEY>, Endpoint: tinc2:655}
/peers/node3 → {IP: 10.0.0.3, Key: <RSA_PUBLIC_KEY>, Endpoint: tinc3:655}
...
```

**How it works:**
1. Forms a cluster of 3-5 nodes (5 in current setup)
2. One node is elected "leader" (automatically)
3. All writes go through the leader
4. Requires majority (quorum) to accept changes
5. If leader fails, new leader is elected in seconds

---

### 4. **Go Daemon (Custom Software) - Go 1.21+**

**What it is:**
- Custom software written in Go programming language
- The "orchestrator" that ties everything together

**What it does:**
- **mDNS Discovery**: Finds other nodes on the network automatically
- **Key Distribution**: Syncs TINC public keys between nodes
- **Connection Management**: Tells TINC which nodes to connect to
- **Health Monitoring**: Watches etcd for changes and reacts

**Why Go?**
- **Cross-platform**: Single binary works on Linux, ARM, x86
- **Low overhead**: <10MB RAM, <1% CPU when idle
- **Concurrency**: Can watch etcd and do mDNS discovery simultaneously (goroutines)
- **Static binary**: No dependencies needed (unlike Python which needs libraries)
- **Fast startup**: <100ms

**Architecture:**
```
daemon-go/
├── cmd/bgp-daemon/main.go       # Entry point, main event loop
├── pkg/
│   ├── discovery/mdns.go        # mDNS peer discovery
│   ├── tinc/manager.go          # TINC configuration management
│   ├── types/types.go           # Data structures (Peer struct)
│   └── metrics/metrics.go       # Prometheus metrics
```

**Main Workflow:**
1. **Startup**: Connect to etcd, read TINC keys, advertise via mDNS
2. **Initial Sync**: Fetch all peers from etcd, sync their host files
3. **Watch Loop**: Monitor etcd for changes (new peers, removed peers)
4. **React**: When a peer joins/leaves, update TINC config and reload daemon
5. **Continuous**: Run mDNS discovery every 30 seconds, expose metrics

---

### 5. **Docker & Docker Compose**

**What it is:**
- Containerization technology
- Docker Compose orchestrates multiple containers

**What it does here:**
- Packages each service (BIRD, TINC, etcd, daemon, monitoring) in isolated containers
- Makes deployment consistent and reproducible
- Simulates a multi-server environment on a single machine

**Container Architecture:**
```
5 TINC containers  → Create mesh VPN
5 BIRD containers  → Run BGP (share network with TINC via network_mode)
5 Go Daemon containers → Orchestrate (share network with TINC)
5 etcd containers  → Store peer info
1 Monitoring container → Prometheus + Grafana
```

**Key Docker Concepts Used:**
- **Multi-stage builds**: Smaller images
- **Network modes**: `network_mode: "service:tinc1"` makes BIRD share TINC's network
- **Volumes**: Persist etcd data, share configs
- **Health checks**: Verify services are working
- **Cap add**: `NET_ADMIN` allows TINC to create network interfaces

---

### 6. **Ansible - Version 2.16+**

**What it is:**
- Infrastructure automation tool
- Uses SSH to configure remote servers

**What it does here:**
- Automates production deployment
- Installs and configures BIRD, TINC, etcd, and daemon on real servers
- Uses templates (Jinja2) to generate configs
- Idempotent: can run multiple times safely

**Structure:**
```
ansible/
├── playbook.yml          # Main playbook (what to do)
├── inventory/
│   └── hosts.ini         # Which servers to configure
├── group_vars/
│   └── all.yml           # Variables (BGP AS, network settings)
└── roles/                # Modular tasks
    ├── bird/             # Install and configure BIRD
    ├── tinc/             # Install and configure TINC
    ├── etcd/             # Install and configure etcd
    └── bgp-daemon/       # Install and configure Go daemon
```

**Deployment modes:**
- **Push mode**: Run from control machine, configures all servers
- **Pull mode** (planned): Servers pull updates from Git every 5 minutes

---

### 7. **Prometheus + Grafana (Monitoring)**

**What it is:**
- Prometheus: Time-series database for metrics
- Grafana: Visualization dashboard

**What it does here:**
- **Prometheus**: Scrapes metrics from BIRD exporter and Go daemon every 15s
- **Grafana**: Displays graphs, alerts, dashboards

**Metrics collected:**
- BGP session states (Established, Idle, Active)
- TINC connection counts
- etcd watch errors
- Peer discovery statistics
- Host file sync duration

**Access:**
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

---

## Architecture & How Everything Works

### Network Topology

```
Physical Network (Internet/LAN)
           │
    ┌──────┴──────┬──────┬──────┬──────┐
    │             │      │      │      │
  Node1         Node2  Node3  Node4  Node5
    │             │      │      │      │
    └─────────────┴──────┴──────┴──────┘
              TINC VPN Mesh
         (10.0.0.1 - 10.0.0.5)
                   │
         BGP Sessions Over Mesh
          (Full mesh topology)
```

### Layered Architecture

```
┌─────────────────────────────────────────┐
│  Application Layer: Go Daemon           │  ← Orchestration
├─────────────────────────────────────────┤
│  Routing Layer: BIRD (BGP)              │  ← Route exchange
├─────────────────────────────────────────┤
│  Transport Layer: TINC (VPN)            │  ← Encrypted tunnels
├─────────────────────────────────────────┤
│  Storage Layer: etcd                    │  ← State storage
├─────────────────────────────────────────┤
│  Monitoring Layer: Prometheus/Grafana   │  ← Observability
├─────────────────────────────────────────┤
│  Orchestration: Docker Compose          │  ← Container management
└─────────────────────────────────────────┘
```

### Complete System Flow

Let me walk through what happens when the system starts:

#### Phase 1: Container Startup (0-20 seconds)

1. **etcd cluster starts first** (dependency)
   - 5 etcd containers start
   - They find each other via initial cluster config
   - Elect a leader using Raft
   - Cluster is ready when quorum (3/5) is healthy

2. **TINC containers start** (depend on etcd)
   - Each container generates RSA-2048 keys (if not existing)
   - Creates host file with public key
   - Starts `tincd` daemon
   - Creates `tinc0` network interface
   - Runs `tinc-up` script:
     - Assigns IP (10.0.0.1, 10.0.0.2, etc.)
     - (Future: stores key in etcd)

3. **BIRD containers start** (share network with TINC)
   - Use `network_mode: "service:tinc1"` (shares tinc1's network stack)
   - Renders config from template (router ID, peers)
   - Starts BIRD daemon
   - Begins establishing BGP sessions over TINC IPs

4. **Go Daemon containers start** (share network with TINC)
   - Connect to etcd
   - Read own TINC public key
   - Store own peer info in etcd: `/peers/node1`
   - Start mDNS advertisement
   - Begin watching etcd for changes

5. **Monitoring starts**
   - Prometheus begins scraping targets
   - Grafana connects to Prometheus
   - Dashboards become available

#### Phase 2: Peer Discovery (20-60 seconds)

1. **Go daemons discover each other**:
   ```
   Daemon1 stores: /peers/node1 → {10.0.0.1, key1, tinc1:655}
   Daemon2 stores: /peers/node2 → {10.0.0.2, key2, tinc2:655}
   Daemon3 stores: /peers/node3 → {10.0.0.3, key3, tinc3:655}
   Daemon4 stores: /peers/node4 → {10.0.0.4, key4, tinc4:655}
   Daemon5 stores: /peers/node5 → {10.0.0.5, key5, tinc5:655}
   ```

2. **Each daemon waits for "calm window"**:
   - Heuristic: Wait until peer count stops changing
   - Max wait: 10 seconds
   - Calm window: 2 seconds with no new peers

3. **Initial sync begins**:
   ```
   For each peer in etcd (except self):
     1. Create/update host file in /var/run/tinc/bgpmesh/hosts/<peer>
     2. Extract node names: node1, node2, node3, node4, node5
     3. Update tinc.conf with ConnectTo directives
     4. Send SIGHUP to tincd (reload config)
   ```

4. **TINC establishes connections**:
   - Each node connects to all others (full mesh)
   - UDP hole punching for NAT traversal
   - Encrypted tunnels established (RSA-2048 + AES-256)
   - Ping test: `10.0.0.1` can reach `10.0.0.2`, `10.0.0.3`, etc.

#### Phase 3: BGP Convergence (60-90 seconds)

1. **BIRD establishes BGP sessions**:
   ```
   bird1 connects to: 10.0.0.2, 10.0.0.3, 10.0.0.4, 10.0.0.5
   bird2 connects to: 10.0.0.1, 10.0.0.3, 10.0.0.4, 10.0.0.5
   ...
   (Full mesh: N*(N-1)/2 sessions = 5*4/2 = 10 sessions total)
   ```

2. **BGP session states**:
   ```
   Idle → Connect → OpenSent → OpenConfirm → Established
   ```

3. **Route exchange**:
   - Each BIRD node advertises its routes
   - Filters apply (filters.conf)
   - Routes installed in kernel routing table

4. **System is converged**:
   - All BGP sessions: Established ✅
   - All TINC connections: Active ✅
   - All peers registered in etcd ✅
   - Monitoring: Collecting metrics ✅

#### Phase 4: Steady State Operations

**Ongoing Activities:**

1. **Go Daemon Event Loop**:
   ```go
   for {
     select {
     case event := <-etcdWatchChannel:
       if event == PUT:
         newPeer := parse(event.data)
         syncHostFile(newPeer)
         reconcileConnections()
         reloadTINC()
       if event == DELETE:
         removeHostFile(deletedPeer)
         reconcileConnections()
         reloadTINC()
     }
   }
   ```

2. **mDNS Discovery** (every 30 seconds):
   - Broadcast: "I'm node1 at 10.0.0.1"
   - Listen for: Other nodes broadcasting
   - Report: Discovered peers count
   - (Currently informational, etcd is source of truth)

3. **BGP Keepalives**:
   - BIRD sends keepalive packets every 60 seconds
   - Detects failures within 180 seconds (or <30s with BFD)

4. **Prometheus Scraping** (every 15 seconds):
   - Queries Go daemon: `http://daemon1:2112/metrics`
   - Queries BIRD exporter (if running)
   - Stores time-series data

5. **Grafana Dashboards**:
   - Refresh every 5 seconds
   - Display: peer counts, BGP states, connection graphs

#### Phase 5: Dynamic Changes

**Scenario: New Node Joins (node6)**

1. **Node6 starts**:
   ```
   docker compose scale tinc=6 bird=6 daemon=6
   ```

2. **Node6 daemon stores key**:
   ```
   etcdctl put /peers/node6 '{"IP":"10.0.0.6","Key":"...","Endpoint":"tinc6:655"}'
   ```

3. **All other daemons receive event**:
   ```
   daemon1: etcd PUT event for /peers/node6
   daemon1: Syncing host file for node6...
   daemon1: Reconciling connections (added: 1, removed: 0)
   daemon1: Reloading TINC...
   ```

4. **TINC connections established**:
   - node1 ↔ node6 tunnel created
   - node2 ↔ node6 tunnel created
   - ... (all nodes connect to node6)

5. **BGP sessions established**:
   - bird1 establishes session with 10.0.0.6
   - bird2 establishes session with 10.0.0.6
   - ...

6. **Total time**: ~30-60 seconds for full convergence

**Scenario: Node Fails (node3 crashes)**

1. **Detection**:
   ```
   - TINC: UDP packets to node3 timeout (no response)
   - BGP: Keepalive timeout after 180s (or 30s with BFD)
   - etcd: Node3 daemon stops updating (lease expires)
   ```

2. **BIRD reacts**:
   ```
   bird1: BGP session to 10.0.0.3 → Idle
   bird1: Removing routes learned from 10.0.0.3
   bird1: Using alternative paths (via node2, node4, node5)
   ```

3. **Optional: etcd cleanup**:
   ```
   # If node3 is truly gone, manually remove:
   etcdctl del /peers/node3
   # All daemons receive DELETE event:
   daemon1: Removing host file for node3
   daemon1: Reconciling connections (added: 0, removed: 1)
   ```

4. **Traffic reroutes**:
   - Packets destined for networks behind node3 reroute
   - Full mesh ensures at least 2 alternative paths
   - Total downtime: 30-180 seconds depending on detection

---

## Component Deep Dive

### 1. BIRD BGP Configuration

**File: `configs/bird/bird.conf.j2`**

```
router id {{ router_id }};          # Unique ID (192.0.2.1, 192.0.2.2, etc.)

log syslog all;                     # Log everything to syslog
debug protocols all;                # Debug BGP protocol

protocol device {                   # Track network interfaces
}

protocol kernel {                   # Sync with Linux kernel routing table
    ipv4 {
        import all;                 # Import routes from kernel
        export all;                 # Export BGP routes to kernel
    };
}

protocol static {                   # Define static routes
    ipv4;
}

include "/etc/bird/protocols.conf"; # BGP peer definitions
include "/etc/bird/filters.conf";  # Route filters
```

**File: `configs/bird/protocols.conf.j2`**

Generated dynamically for each node:

```jinja2
{% for peer_id in range(1, total_nodes + 1) %}
{% if peer_id != node_id %}
protocol bgp peer{{ loop.index }} {
    description "BGP peer at 10.0.0.{{ peer_id }}";
    local {{ node_ip }} as {{ bgp_as }};    # Our IP and AS number
    neighbor 10.0.0.{{ peer_id }} as {{ bgp_as }};  # Peer IP and AS (iBGP)
    
    ipv4 {
        import all;   # Accept all routes from peer
        export all;   # Advertise all routes to peer
    };
}
{% endif %}
{% endfor %}
```

For node1 (5-node setup), this generates:
```
protocol bgp peer1 { neighbor 10.0.0.2 as 65000; }
protocol bgp peer2 { neighbor 10.0.0.3 as 65000; }
protocol bgp peer3 { neighbor 10.0.0.4 as 65000; }
protocol bgp peer4 { neighbor 10.0.0.5 as 65000; }
```

**Key BGP Concepts:**

- **AS (Autonomous System)**: All nodes use AS 65000 (iBGP - internal BGP)
- **Router ID**: Unique identifier (uses 192.0.2.x range for clarity)
- **Full mesh**: Every node peers with every other node
- **iBGP**: Internal BGP (same AS number) for route distribution within mesh

---

### 2. TINC VPN Configuration

**File: `configs/tinc/tinc.conf.j2`**

```jinja2
Name = {{ tinc_name }}       # node1, node2, etc.
Device = /dev/net/tun        # TUN device
Mode = switch                # Layer 2 switch mode (acts like a network switch)
Port = {{ tinc_port }}       # UDP port (default 655)

# ConnectTo directives added dynamically by Go daemon
# ConnectTo = node2
# ConnectTo = node3
# ...
```

**Mode: switch vs router:**
- **switch mode**: Layer 2, nodes appear on same subnet (10.0.0.0/24)
  - BGP packets are Ethernet frames
  - Works like a virtual switch
- **router mode**: Layer 3, each node has own subnet
  - Would require routing between subnets
  - More complex for this use case

**File: Host files** (`/var/run/tinc/bgpmesh/hosts/node1`)

```
Address = tinc1      # DNS name or IP
Port = 655           # UDP port
Subnet = 10.0.0.1/32 # IP address for this node

-----BEGIN RSA PUBLIC KEY-----
<public key data>
-----END RSA PUBLIC KEY-----
```

**How TINC Establishes Connections:**

1. Read `tinc.conf`: See `ConnectTo = node2`
2. Look up `hosts/node2`: Find `Address = tinc2`, `Port = 655`
3. Resolve DNS: `tinc2` → `172.20.0.3` (Docker internal IP)
4. Initiate UDP connection: Send handshake packet
5. Exchange: Protocol version, node names
6. Authenticate: Verify public key signatures
7. Establish: Create encrypted tunnel with AES-256
8. Subnet assignment: node2 owns `10.0.0.2/32`
9. L2 switching: Forward Ethernet frames via tunnel

**File: `tinc-up` script**

```bash
#!/bin/sh
ip link set $INTERFACE up mtu 1400
ip addr add 10.0.0.$NODE_ID/24 dev $INTERFACE
# Future: etcdctl put /peers/$TINC_NAME "$(tinc info)"
```

---

### 3. etcd Cluster Configuration

**Docker Compose Config:**

```yaml
etcd1:
  image: quay.io/coreos/etcd:v3.5.14
  command:
    - etcd
    - --name=etcd1                              # Node name
    - --data-dir=/etcd-data                     # Data directory
    - --listen-client-urls=http://0.0.0.0:2379 # API port
    - --advertise-client-urls=http://etcd1:2379
    - --listen-peer-urls=http://0.0.0.0:2380   # Raft port
    - --initial-advertise-peer-urls=http://etcd1:2380
    - --initial-cluster=etcd1=http://etcd1:2380,etcd2=http://etcd2:2380,...
    - --initial-cluster-state=new               # Bootstrap new cluster
```

**Raft Consensus Algorithm:**

```
1. Leader Election:
   - All nodes start as followers
   - If no leader after timeout, node becomes candidate
   - Candidate requests votes from other nodes
   - Node with majority votes becomes leader

2. Log Replication:
   - All writes go through leader
   - Leader appends to its log
   - Leader replicates to followers
   - Once majority confirm, entry is committed
   - Leader notifies followers of commit

3. Fault Tolerance:
   - 5 nodes: tolerates 2 failures (needs 3 for quorum)
   - 3 nodes: tolerates 1 failure (needs 2 for quorum)
   - If leader fails, new election in <5 seconds
```

**API Usage:**

```bash
# Store peer info
etcdctl put /peers/node1 '{"IP":"10.0.0.1","Key":"...","Endpoint":"tinc1:655"}'

# Get all peers
etcdctl get /peers/ --prefix

# Watch for changes (Go daemon uses this)
etcdctl watch /peers/ --prefix

# Delete peer
etcdctl del /peers/node1
```

---

### 4. Go Daemon Architecture

**Package Structure:**

```
pkg/
├── types/        # Data structures
│   └── types.go
│       type Peer struct {
│           IP       net.IP  # TINC mesh IP (10.0.0.x)
│           Key      string  # RSA public key
│           Endpoint string  # Docker hostname:port (tinc2:655)
│       }
│
├── discovery/    # mDNS peer discovery
│   └── mdns.go
│       - LookupPeers(iface string) []Peer
│       - AdvertiseService(name, port, key)
│       - MonitorPeers(ctx, iface, interval, callback)
│
├── tinc/         # TINC configuration management
│   └── manager.go
│       - SyncHostFile(nodeName, peer)          # Create/update host file
│       - RemoveHostFile(nodeName)              # Delete host file
│       - ReconcileConnections(desiredPeers)    # Update tinc.conf
│       - Reload()                              # SIGHUP to tincd
│
└── metrics/      # Prometheus metrics
    └── metrics.go
        - PeersDiscovered      (gauge)
        - TincConnectionsActive (gauge)
        - PeerSyncTotal        (counter)
        - HostFileSyncDuration (histogram)
```

**Main Loop (`cmd/bgp-daemon/main.go`):**

```go
// Simplified version

func main() {
    // 1. Setup
    etcdClient := connectToEtcd()
    tincManager := tinc.NewManager("bgpmesh")
    
    // 2. Read own key and store in etcd
    localKey := tincManager.GetPublicKey(nodeName)
    etcdClient.Put("/peers/" + nodeName, peerJSON)
    
    // 3. Start mDNS advertisement
    mdnsServer := discovery.AdvertiseService(nodeName, 655, keyFingerprint)
    
    // 4. Initial peer sync (with "calm window" heuristic)
    waitForPeerStability()  // Wait until peer count stabilizes
    peers := etcdClient.Get("/peers/", WithPrefix())
    for _, peer := range peers {
        tincManager.SyncHostFile(peer.Name, peer)
    }
    tincManager.ReconcileConnections(allPeerNames)
    
    // 5. Watch etcd for changes
    watchChan := etcdClient.Watch("/peers/", WithPrefix())
    
    // 6. Event loop
    for {
        select {
        case event := <-watchChan:
            switch event.Type {
            case PUT:
                newPeer := parseEvent(event)
                tincManager.SyncHostFile(newPeer.Name, newPeer)
                reconcileAllConnections()
            case DELETE:
                tincManager.RemoveHostFile(event.Key)
                reconcileAllConnections()
            }
        }
    }
}

func reconcileAllConnections() {
    // Get all current peers from etcd
    allPeers := etcdClient.Get("/peers/", WithPrefix())
    peerNames := extractNames(allPeers)
    
    // Update tinc.conf with full list and reload
    tincManager.ReconcileConnections(peerNames)
}
```

**TINC Connection Reconciliation (Full Mesh Logic):**

```go
func (m *Manager) ReconcileConnections(desiredPeers []string) (int, int, error) {
    // 1. Read current connections from tinc.conf
    current := m.GetCurrentConnections()  // ["node2", "node3"]
    
    // 2. Calculate diff
    added := 0
    removed := 0
    for _, peer := range desiredPeers {
        if !contains(current, peer) {
            added++  // New peer to connect
        }
    }
    for _, peer := range current {
        if !contains(desiredPeers, peer) {
            removed++  // Old peer to disconnect
        }
    }
    
    // 3. Update tinc.conf (replace all ConnectTo lines)
    m.UpdateConnectTo(desiredPeers)
    // Before:
    // Name = node1
    // Mode = switch
    // ConnectTo = node2
    // ConnectTo = node3
    //
    // After (if node4 joined):
    // Name = node1
    // Mode = switch
    // ConnectTo = node2
    // ConnectTo = node3
    // ConnectTo = node4
    
    // 4. Reload TINC daemon (SIGHUP)
    m.Reload()  // Send kill -HUP <tincd_pid>
    
    return added, removed, nil
}
```

**Shared PID Namespace:**

The daemon shares the PID namespace with TINC container:

```yaml
daemon1:
  network_mode: "service:tinc1"   # Share network
  pid: "service:tinc1"             # Share PID namespace
```

This allows the daemon to:
- See `tincd` process: `pidof tincd` works
- Send signals: `kill -HUP <pid>` works
- No need for remote API or file-based triggers

---

### 5. Docker Compose Architecture

**Network Topology:**

```yaml
networks:
  mesh-net:           # For Docker service discovery (tinc1, tinc2, etc.)
    driver: bridge
    subnet: 172.20.0.0/16
  cluster-net:        # For etcd cluster (internal only)
    driver: bridge
    internal: true    # No external access
```

**Service Dependencies:**

```
Dependency Graph:
├── etcd1, etcd2, etcd3, etcd4, etcd5  (independent cluster)
├── tinc1, tinc2, tinc3, tinc4, tinc5  (depend on etcd)
├── bird1, bird2, bird3, bird4, bird5  (depend on tinc, share network)
├── daemon1, daemon2, ..., daemon5     (depend on tinc, share network & PID)
└── prometheus                          (scrapes all)
```

**Shared Network Mode:**

```yaml
tinc1:
  container_name: tinc1
  networks:
    - mesh-net      # Can reach other containers
  ports:
    - "655:655/udp" # Expose UDP port
    - "179:179"     # Expose BGP port (for bird1)

bird1:
  container_name: bird1
  network_mode: "service:tinc1"  # Share tinc1's network stack
  # No separate network config needed
  # bird1 uses tinc1's IPs, ports, interfaces

daemon1:
  container_name: daemon1
  network_mode: "service:tinc1"  # Share tinc1's network stack
  pid: "service:tinc1"            # Share tinc1's PID namespace
```

**Why this design?**

- BIRD needs to see `tinc0` interface (only exists in TINC's network namespace)
- BIRD needs to bind to TINC's IP addresses (10.0.0.x)
- Daemon needs to reload TINC (needs PID access)
- Simpler than inter-process communication or APIs

**Volume Mounts:**

```yaml
bird1:
  volumes:
    - ./configs/bird:/etc/bird:ro    # Read-only config templates

tinc1:
  volumes:
    - ./configs/tinc:/etc/tinc:ro    # Read-only config templates
    - tinc1-data:/var/run/tinc       # Persistent keys and runtime files

etcd1:
  volumes:
    - etcd1-data:/etcd-data          # Persistent database

volumes:
  etcd1-data:   # Named volume (persists between restarts)
  tinc1-data:
  # ...
```

---

### 6. Ansible Automation

**Role Structure:**

Each role follows Ansible Galaxy standards:

```
roles/bird/
├── defaults/main.yml      # Default variables
├── handlers/main.yml      # Actions triggered by changes
├── meta/main.yml          # Role metadata
├── tasks/main.yml         # Main tasks
└── templates/             # Jinja2 templates
    ├── bird.conf.j2
    └── protocols.conf.j2
```

**Example: BIRD Role (`roles/bird/tasks/main.yml`):**

```yaml
---
- name: Install BIRD
  apt:
    name: bird2       # BIRD 3.x in Debian 12
    state: present
  become: yes

- name: Create BIRD config directory
  file:
    path: /etc/bird
    state: directory
    mode: '0755'

- name: Template BIRD main config
  template:
    src: bird.conf.j2
    dest: /etc/bird/bird.conf
    mode: '0644'
  notify: restart bird    # Triggers handler

- name: Template BIRD protocols
  template:
    src: protocols.conf.j2
    dest: /etc/bird/protocols.conf
    mode: '0644'
  notify: restart bird

- name: Enable and start BIRD service
  systemd:
    name: bird
    enabled: yes
    state: started
  become: yes
```

**Handler (`roles/bird/handlers/main.yml`):**

```yaml
---
- name: restart bird
  systemd:
    name: bird
    state: restarted
  become: yes
```

**Variables (`group_vars/all.yml`):**

```yaml
---
# BGP configuration
bgp_as: 65000
router_id_prefix: "192.0.2"

# TINC configuration
tinc_netname: bgpmesh
tinc_port: 655

# etcd configuration
etcd_cluster_token: "bgp-mesh-cluster"
etcd_endpoints:
  - http://10.1.1.1:2379
  - http://10.1.1.2:2379
  - http://10.1.1.3:2379
```

**Inventory (`inventory/hosts.ini`):**

```ini
[bgp_nodes]
node1 ansible_host=10.1.1.1 router_id=192.0.2.1 node_ip=10.0.0.1
node2 ansible_host=10.1.1.2 router_id=192.0.2.2 node_ip=10.0.0.2
node3 ansible_host=10.1.1.3 router_id=192.0.2.3 node_ip=10.0.0.3

[etcd_nodes]
node1
node2
node3

[tinc_nodes]
node1
node2
node3
```

**Playbook (`playbook.yml`):**

```yaml
---
- name: Deploy BGP mesh infrastructure
  hosts: bgp_nodes
  become: yes
  roles:
    - etcd        # Install and configure etcd
    - tinc        # Install and configure TINC VPN
    - bird        # Install and configure BIRD BGP
    - bgp-daemon  # Install and configure Go daemon
```

**Running Ansible:**

```bash
# Check syntax
ansible-playbook playbook.yml --syntax-check

# Dry run (show what would change)
ansible-playbook playbook.yml --check --diff

# Execute
ansible-playbook playbook.yml -i inventory/hosts.ini

# Execute with verbose output
ansible-playbook playbook.yml -vvv

# Execute on specific nodes
ansible-playbook playbook.yml --limit node1,node2
```

---

### 7. Monitoring with Prometheus & Grafana

**Prometheus Configuration (`configs/prometheus/prometheus.yml`):**

```yaml
global:
  scrape_interval: 15s      # Scrape targets every 15 seconds
  evaluation_interval: 15s  # Evaluate rules every 15 seconds

scrape_configs:
  - job_name: 'bgp-daemons'
    static_configs:
      - targets:
          - 'daemon1:2112'   # Go daemon metrics endpoint
          - 'daemon2:2112'
          - 'daemon3:2112'
          - 'daemon4:2112'
          - 'daemon5:2112'
  
  # Future: BIRD exporter
  - job_name: 'bird-exporters'
    static_configs:
      - targets:
          - 'bird1:9324'
          - 'bird2:9324'
          # ...
```

**Metrics Exposed by Go Daemon:**

```
# HELP bgp_daemon_peers_discovered Number of peers discovered via mDNS
# TYPE bgp_daemon_peers_discovered gauge
bgp_daemon_peers_discovered 4

# HELP bgp_daemon_peer_sync_total Total peer sync operations
# TYPE bgp_daemon_peer_sync_total counter
bgp_daemon_peer_sync_total{status="success",operation="PUT"} 15
bgp_daemon_peer_sync_total{status="error",operation="PUT"} 0

# HELP bgp_daemon_tinc_connections_active Active TINC connections
# TYPE bgp_daemon_tinc_connections_active gauge
bgp_daemon_tinc_connections_active 4

# HELP bgp_daemon_host_file_sync_duration_seconds Time to sync host file
# TYPE bgp_daemon_host_file_sync_duration_seconds histogram
bgp_daemon_host_file_sync_duration_seconds_bucket{le="0.005"} 10
bgp_daemon_host_file_sync_duration_seconds_bucket{le="0.01"} 25
# ...
```

**Grafana Dashboard Structure:**

```
BGP Daemon Overview Dashboard
├── Panel 1: Peer Discovery
│   └── Graph: bgp_daemon_peers_discovered (all nodes)
├── Panel 2: TINC Connections
│   └── Graph: bgp_daemon_tinc_connections_active
├── Panel 3: Sync Operations
│   └── Counter: bgp_daemon_peer_sync_total (success vs error)
├── Panel 4: Host File Sync Latency
│   └── Histogram: bgp_daemon_host_file_sync_duration_seconds
└── Panel 5: etcd Watch Errors
    └── Counter: bgp_daemon_etcd_watch_errors_total
```

**Accessing Monitoring:**

```bash
# Prometheus (raw metrics and queries)
open http://localhost:9090

# Example queries:
# - Rate of sync operations: rate(bgp_daemon_peer_sync_total[5m])
# - 95th percentile sync time: histogram_quantile(0.95, bgp_daemon_host_file_sync_duration_seconds_bucket)

# Grafana (dashboards)
open http://localhost:3000
# Login: admin / admin
# Navigate: Dashboards → BGP Daemon Overview
```

---

## File Structure Explained

### Root Directory

```
BGP4mesh-fork-santi/
├── README.md                 # Project overview, quick start
├── Arquitectura.md           # Architecture details (Spanish)
├── CLAUDE.md                 # AI development notes
├── Makefile                  # Build and deployment automation
├── docker-compose.yml        # Container orchestration (15 services)
├── tinc_bootstrap.sh         # Legacy bootstrap script
├── PLAN-OPTIMIZADO-GROK.md   # Project planning
├── STATUS-*.md               # Sprint status reports
└── PROMPT-BGP-NETWORK.md     # Original project prompt
```

### configs/ - Configuration Templates

```
configs/
├── bird/                     # BIRD BGP configs
│   ├── bird.conf.j2          # Main config (Jinja2 template)
│   ├── protocols.conf.j2     # BGP peer definitions (templated)
│   ├── protocols-*.conf      # Static examples
│   └── filters.conf          # Route filters (static)
│
├── tinc/                     # TINC VPN configs
│   ├── tinc.conf.j2          # Main config (templated)
│   ├── tinc-up.j2            # Interface up script (templated)
│   └── tinc-down.j2          # Interface down script (templated)
│
├── etcd/                     # etcd configs
│   └── etcd.conf             # Basic cluster config
│
├── prometheus/               # Monitoring configs
│   └── prometheus.yml        # Scrape targets
│
└── grafana/                  # Dashboard configs
    ├── dashboards/           # Dashboard JSON definitions
    │   └── bgp-daemon-overview.json
    └── provisioning/         # Auto-load configs
        ├── dashboards/
        │   └── dashboards.yml
        └── datasources/
            └── prometheus.yml
```

**Why Jinja2 templates (.j2)?**
- Variables: `{{ node_ip }}`, `{{ bgp_as }}`
- Loops: Generate N peer configs automatically
- Conditionals: Different configs per node type
- Reusable: Same template for Docker and Ansible

### docker/ - Container Definitions

```
docker/
├── bird/                     # BIRD container
│   ├── Dockerfile            # FROM debian:12-slim, install bird2
│   └── entrypoint.sh         # Render templates, start bird
│
├── tinc/                     # TINC container
│   ├── Dockerfile            # FROM debian:12-slim, install tinc
│   └── entrypoint.sh         # Generate keys, render configs, start tincd
│
├── go-daemon/                # Go daemon container
│   └── Dockerfile            # Multi-stage: build Go binary, minimal runtime
│
└── monitoring/               # Prometheus + Grafana
    ├── Dockerfile            # FROM prom + grafana, supervisord
    └── entrypoint.sh         # Start both services
```

### daemon-go/ - Custom Orchestration Software

```
daemon-go/
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
├── Makefile                  # Build, test, coverage targets
├── README.md                 # Daemon-specific docs
│
├── cmd/                      # Executables
│   └── bgp-daemon/
│       └── main.go           # Entry point (494 lines)
│
└── pkg/                      # Reusable packages
    ├── discovery/            # mDNS peer discovery
    │   ├── mdns.go           # Service advertisement and lookup
    │   └── mdns_test.go      # Unit tests (89.8% coverage)
    │
    ├── tinc/                 # TINC configuration management
    │   ├── manager.go        # File operations, reload logic
    │   └── manager_test.go   # Unit tests (92.7% coverage)
    │
    ├── types/                # Data structures
    │   ├── types.go          # Peer struct
    │   └── types_test.go     # Unit tests (100% coverage)
    │
    └── metrics/              # Prometheus metrics
        ├── metrics.go        # Metric definitions
        └── metrics_test.go   # Unit tests
```

**Test Coverage:**
- Run: `cd daemon-go && make test-coverage`
- View: `make test-coverage-html` (opens browser)
- CI enforcement: Fails if <80%

### ansible/ - Infrastructure Automation

```
ansible/
├── ansible.cfg               # Ansible settings
├── playbook.yml              # Main playbook (calls all roles)
├── site.yml                  # Alternative entry point
│
├── inventory/                # Target hosts
│   ├── hosts.ini             # Production inventory
│   ├── hosts.ini.example     # Template
│   └── group_vars/
│       └── bgp_nodes.yml     # Node-specific variables
│
├── group_vars/               # Global variables
│   └── all.yml               # BGP AS, network settings
│
└── roles/                    # Modular tasks
    ├── bird/                 # BIRD installation and configuration
    ├── tinc/                 # TINC installation and configuration
    ├── etcd/                 # etcd installation and configuration
    └── bgp-daemon/           # Go daemon deployment
        ├── tasks/main.yml
        ├── templates/
        │   ├── bgp-daemon.service.j2    # systemd unit
        │   └── bgp-daemon.env.j2        # Environment file
        └── defaults/main.yml
```

### tests/ - Validation and Testing

```
tests/
├── validation/               # Fast pre-flight checks
│   ├── test_env_vars.sh      # Check required environment variables
│   ├── test_configs.sh       # Validate Jinja2 templates render correctly
│   └── test_docker_builds.sh # Test Docker images build successfully
│
├── integration/              # Service integration tests
│   └── test_bgp_peering.sh   # Verify BGP sessions, TINC connectivity, etcd health
│
└── e2e/                      # End-to-end workflows
    └── test_full_stack.sh    # Full deployment → convergence → verification
```

**Test Execution:**

```bash
# All tests (parallel validation, then integration, then E2E)
make test-all

# Individual suites
make test-env          # <5 seconds
make test-configs      # ~10 seconds
make test-builds       # ~60 seconds (builds 3 images)
make test-integration  # ~90 seconds (requires running stack)
make test-e2e          # ~120 seconds (full deploy + teardown)
```

### docs/ - Documentation

```
docs/
├── QUICKSTART.md             # Getting started guide
├── DEPLOYMENT.md             # Production deployment guide
├── MANUAL_TESTING.md         # Manual verification steps
├── TESTING.md                # Testing strategy and coverage
│
└── architecture/
    └── decisions.md          # Architecture Decision Records (ADRs)
                              # - ADR-001: BIRD 3.x choice
                              # - ADR-002: TINC 1.0 choice
                              # - ADR-003: etcd choice
                              # - ...
```

### scripts/ - Utilities

```
scripts/
├── install-hooks.sh          # Install git hooks (linting, pre-commit)
└── README.md                 # Script documentation
```

---

## How to Use This Project

### Prerequisites

Install these on your system:

```bash
# Docker and Docker Compose
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER  # Add your user to docker group
newgrp docker                  # Activate group

# Verify
docker --version     # Should be 24.0+
docker compose version  # Should be v2.0+

# Go (for daemon development)
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/bin/go/bin' >> ~/.bashrc
source ~/.bashrc
go version  # Should be 1.21+

# Ansible (for production deployment)
sudo apt update
sudo apt install -y ansible
ansible --version  # Should be 2.16+
```

### Quick Start: 5-Node Local Deployment

**Step 1: Clone and Setup**

```bash
cd ~/repos
git clone <repository-url> BGP4mesh
cd BGP4mesh

# Optional: Create .env (uses defaults if not present)
cp .env.example .env
vim .env  # Customize if needed
```

**Step 2: Deploy**

```bash
make deploy-local
```

This will:
1. Build Docker images (~2-3 minutes first time)
2. Start 20 containers:
   - 5 etcd (cluster-net)
   - 5 tinc (mesh-net)
   - 5 bird (share tinc network)
   - 5 daemon (share tinc network)
   - 1 prometheus+grafana
3. Bootstrap etcd cluster
4. Generate TINC keys
5. Wait for convergence (~90 seconds)

**Step 3: Verify**

```bash
# Check all containers running
docker ps
# Should see 20 containers, all "Up"

# Check BGP sessions
docker exec bird1 birdc show protocols
# Look for "BGP", "Established" (should be 4 sessions per node)

# Check TINC connectivity
docker exec tinc1 ping -c 3 10.0.0.2
docker exec tinc1 ping -c 3 10.0.0.5
# Should have replies

# Check etcd cluster
docker exec etcd1 etcdctl endpoint health --endpoints=etcd1:2379,etcd2:2379,etcd3:2379,etcd4:2379,etcd5:2379
# All endpoints should be "healthy"

# Check daemon logs
docker logs daemon1 | tail -20
# Should see: "✓ Daemon running"

# View all peer registrations
docker exec etcd1 etcdctl get /peers/ --prefix
# Should list /peers/node1 through /peers/node5
```

**Step 4: Monitor**

```bash
make monitor
# Opens Grafana at http://localhost:3000

# Login: admin / admin
# Navigate: Dashboards → BGP Daemon Overview

# Also available:
# Prometheus: http://localhost:9090
```

**Step 5: Run Tests**

```bash
make test-all
# Runs validation, integration, and E2E tests
# Should see all tests PASS
```

**Step 6: Teardown**

```bash
make clean
# Stops and removes all containers, networks, volumes
```

### Manual Commands

**BIRD (BGP) Commands:**

```bash
# Show all protocols
docker exec bird1 birdc show protocols

# Show detailed protocol info
docker exec bird1 birdc show protocols all peer1

# Show BGP route table
docker exec bird1 birdc show route all

# Show route for specific destination
docker exec bird1 birdc show route for 10.0.0.3

# Reload BIRD config (without restart)
docker exec bird1 birdc configure
```

**TINC Commands:**

```bash
# Show TINC info
docker exec tinc1 tinc -n bgpmesh info

# List all nodes
docker exec tinc1 tinc -n bgpmesh dump nodes

# Show connections
docker exec tinc1 tinc -n bgpmesh dump edges

# Show subnet assignments
docker exec tinc1 tinc -n bgpmesh dump subnets

# Check interface
docker exec tinc1 ip addr show tinc0
```

**etcd Commands:**

```bash
# List all peers
docker exec etcd1 etcdctl get /peers/ --prefix

# Get specific peer
docker exec etcd1 etcdctl get /peers/node1

# Watch for changes (real-time)
docker exec etcd1 etcdctl watch /peers/ --prefix

# Check cluster members
docker exec etcd1 etcdctl member list

# Check cluster health
docker exec etcd1 etcdctl endpoint health

# Check cluster status
docker exec etcd1 etcdctl endpoint status --write-out=table
```

**Daemon Logs:**

```bash
# Follow daemon logs
docker logs -f daemon1

# Last 50 lines
docker logs --tail 50 daemon1

# Search for errors
docker logs daemon1 | grep -i error

# View all daemon logs simultaneously
docker compose logs -f daemon1 daemon2 daemon3 daemon4 daemon5
```

**Network Debugging:**

```bash
# Ping test (via TINC mesh)
docker exec tinc1 ping -c 3 10.0.0.2
docker exec tinc1 ping -c 3 10.0.0.5

# Traceroute
docker exec tinc1 traceroute 10.0.0.5

# Check routing table
docker exec bird1 ip route

# Check network interfaces
docker exec tinc1 ip addr

# Check UDP ports
docker exec tinc1 netstat -uln | grep 655

# TCP connections
docker exec bird1 netstat -tn | grep 179
```

---

## Development Workflow

### Modifying BIRD Configuration

```bash
# 1. Edit template
vim configs/bird/bird.conf.j2
# Or
vim configs/bird/protocols.conf.j2

# 2. Validate template syntax
make test-configs

# 3. Restart BIRD containers to apply changes
docker restart bird1 bird2 bird3 bird4 bird5

# 4. Verify
docker exec bird1 birdc show protocols
docker logs bird1 | tail -20
```

### Modifying TINC Configuration

```bash
# 1. Edit template
vim configs/tinc/tinc.conf.j2
# Or
vim configs/tinc/tinc-up.j2

# 2. Rebuild and restart TINC containers
docker compose up -d --build tinc1 tinc2 tinc3 tinc4 tinc5

# 3. Verify
docker exec tinc1 cat /var/run/tinc/bgpmesh/tinc.conf
docker exec tinc1 ip addr show tinc0
```

### Modifying Go Daemon

```bash
# 1. Edit source code
cd daemon-go
vim pkg/tinc/manager.go
# Or
vim cmd/bgp-daemon/main.go

# 2. Run tests locally
make test
make test-coverage

# 3. Build binary
make build
# Produces: daemon-go/bgp-daemon

# 4. Rebuild Docker image
cd ..
docker compose up -d --build daemon1 daemon2 daemon3 daemon4 daemon5

# 5. Verify
docker logs -f daemon1
```

### Adding a New Node

```bash
# Scale up (adds node6)
docker compose up -d --scale tinc=6 --scale bird=6 --scale daemon=6 --scale etcd=6

# Verify convergence
docker logs daemon1 | grep node6
docker exec bird1 birdc show protocols | grep peer
docker exec etcd1 etcdctl get /peers/node6
```

### Simulating Failures (Chaos Testing)

```bash
# Kill a node
docker stop tinc3 bird3 daemon3

# Observe logs on other nodes
docker logs -f daemon1

# Check BGP reconvergence
docker exec bird1 birdc show protocols
# peer3 should show "Idle" or "Connect"

# Check routing still works
docker exec tinc1 ping -c 3 10.0.0.5
# Should work (routes via other nodes)

# Bring node back
docker start tinc3 bird3 daemon3

# Observe recovery
docker logs -f daemon1
# Should see: "etcd PUT event for /peers/node3"
```

---

## Key Concepts for Beginners

### 1. What is BGP?

**Border Gateway Protocol** - The protocol that runs the Internet.

**Analogy:**
- Think of the Internet as a road network
- BGP is like GPS navigation systems telling each other about roads
- Each router says "I know how to reach 10.0.0.1, it's 2 hops away"
- Other routers update their maps based on this info

**In this project:**
- Each BIRD instance is a BGP router
- They exchange routes over the TINC mesh
- If a path fails, BGP recalculates alternative paths

**Key terms:**
- **AS (Autonomous System)**: A network under single administrative control (we use AS 65000)
- **Peer**: Another BGP router we exchange routes with
- **Route**: "To reach 10.0.0.3, send packets to next hop 10.0.0.2"
- **Session**: A TCP connection between two BGP routers

### 2. What is a VPN?

**Virtual Private Network** - An encrypted tunnel between two computers.

**Analogy:**
- Like a private underground tunnel between your houses
- Only you and your friends can use it
- Even if someone intercepts traffic, it's encrypted (unreadable)

**In this project:**
- TINC creates VPN tunnels between all nodes
- Forms a mesh topology (everyone connected to everyone)
- All traffic is encrypted with AES-256
- Operates at Layer 2 (like a virtual switch)

**Key terms:**
- **Mesh**: Every node connects to every other node (N*(N-1)/2 connections)
- **Tunnel**: Encrypted connection between two nodes
- **Switch mode**: Acts like a network switch (Layer 2)
- **tun0/tinc0**: Virtual network interface created by TINC

### 3. What is etcd?

**Distributed database** - Like a spreadsheet that multiple servers share.

**Analogy:**
- Google Sheets where everyone can edit simultaneously
- Changes sync to everyone in real-time
- Uses voting to prevent conflicts (Raft algorithm)

**In this project:**
- Stores information about all nodes
- Each daemon writes its own info
- Each daemon watches for changes from others
- Enables automatic peer discovery

**Key terms:**
- **Key-value store**: Data organized as key → value pairs
- **Watch**: Get notified when data changes
- **Quorum**: Majority vote (3 out of 5 nodes must agree)
- **Raft**: Algorithm for distributed consensus

### 4. What is Docker?

**Containerization** - Like lightweight virtual machines.

**Analogy:**
- Virtual machines are entire houses
- Containers are rooms in a house (share foundation)
- Much lighter and faster than VMs

**In this project:**
- Each service runs in its own container
- Containers are isolated but can communicate
- Docker Compose orchestrates multiple containers
- Simulates a multi-server environment on one machine

**Key terms:**
- **Image**: Template for a container (like an app installer)
- **Container**: Running instance of an image (like an app)
- **Volume**: Persistent storage (survives container restarts)
- **Network**: Virtual network connecting containers

### 5. What is mDNS?

**Multicast DNS** - Automatic device discovery on local networks.

**Analogy:**
- Like shouting "Is anyone named Bob here?" in a room
- Bob responds "I'm Bob, I'm at table 5"
- No central directory needed

**In this project:**
- Daemons broadcast "I'm node1 at 10.0.0.1"
- Other daemons discover them automatically
- Backup to etcd discovery method

**Key terms:**
- **Multicast**: One-to-many communication
- **Service discovery**: Finding other services on the network
- **.local**: Special domain for mDNS (e.g., node1.local)

### 6. What is Jinja2?

**Templating language** - Like mail merge for config files.

**Example:**

Template:
```jinja2
Hello {{ name }}, you are {{ age }} years old.
```

Data:
```
name = "Alice"
age = 30
```

Result:
```
Hello Alice, you are 30 years old.
```

**In this project:**
- Generate BIRD configs for each node
- Same template, different variables per node
- Used by both Docker (entrypoint.sh) and Ansible

### 7. What is Ansible?

**Configuration management** - Like a recipe for server setup.

**Analogy:**
- Chef's recipe: "Add 2 cups flour, mix, bake 350°F"
- Ansible playbook: "Install BIRD, configure, start service"
- Idempotent: Can run multiple times safely (like "ensure oven is 350°F" vs "turn oven up 50°F")

**In this project:**
- Automates production deployment
- Connects to servers via SSH
- Runs tasks in order
- Uses same config templates as Docker

---

## Testing Infrastructure

### Test Pyramid

```
       E2E Tests (Full Stack)
      /                    \
     /  Integration Tests   \
    /  (BGP, TINC, etcd)     \
   /____________________________\
  /  Validation Tests            \
 /  (Env, Configs, Builds)        \
/____________________________________\
  Unit Tests (Go daemon packages)
```

### Test Types

**1. Unit Tests (Go daemon)**

Location: `daemon-go/pkg/*/`

```bash
cd daemon-go

# Run all tests
make test

# With coverage
make test-coverage

# Coverage report
make test-coverage-html
```

Example test:
```go
func TestPeerIsValid(t *testing.T) {
    peer := types.Peer{
        IP:       net.ParseIP("10.0.0.1"),
        Endpoint: "tinc1:655",
    }
    
    if !peer.IsValid() {
        t.Error("Expected peer to be valid")
    }
}
```

**2. Validation Tests**

Location: `tests/validation/`

Purpose: Fast pre-flight checks

```bash
# Environment variables
./tests/validation/test_env_vars.sh
# Checks: Docker available, docker-compose version, etc.

# Configuration templates
./tests/validation/test_configs.sh
# Checks: Jinja2 templates render without errors

# Docker builds
./tests/validation/test_docker_builds.sh
# Checks: All Dockerfiles build successfully
```

**3. Integration Tests**

Location: `tests/integration/`

Purpose: Verify services work together

```bash
./tests/integration/test_bgp_peering.sh
```

Verifies:
- BGP sessions reach "Established" state
- TINC tunnels are active
- etcd cluster is healthy
- Peer data is synced
- Network connectivity works (ping test)

**4. E2E Tests**

Location: `tests/e2e/`

Purpose: Full workflow from scratch

```bash
./tests/e2e/test_full_stack.sh
```

Flow:
1. `make clean` (teardown any existing)
2. `make deploy-local` (deploy from scratch)
3. Wait for convergence (90s)
4. Run all integration checks
5. Simulate failure (stop node)
6. Verify recovery
7. `make clean` (teardown)

### Coverage Targets

- **Unit tests**: >80% (currently 92.7% for tinc, 89.8% for discovery)
- **Integration tests**: 100% of critical paths
- **E2E tests**: 100% of user workflows

### CI Integration (Future)

Planned GitHub Actions workflow:

```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      
      - name: Go unit tests
        run: cd daemon-go && make test-coverage
      
      - name: Validation tests
        run: make test-fast
      
      - name: Build images
        run: make test-builds
      
      - name: Integration tests
        run: |
          make deploy-local
          make test-integration
          make clean
```

---

## Future Roadmap

### Sprint 2 Phase 2 (Current)

**Goals:**
- Complete unit test coverage (>90% all packages)
- Custom Grafana dashboards
- Additional integration tests
- Performance benchmarking

**Deliverables:**
- `make test-coverage` reports >90%
- Grafana dashboard showing BGP session states
- Integration test for node failure scenarios
- Benchmark: <30s reconvergence with BFD

### Sprint 3: Production Hardening

**Goals:**
- systemd service units for production
- Secrets management (Ansible Vault)
- Rolling updates without downtime
- Chaos testing (automated failure injection)
- BGP MD5 or TCP-AO authentication

**Deliverables:**
- Ansible playbook for production deployment
- systemd units for BIRD, TINC, etcd, daemon
- Vault-encrypted secrets (BGP passwords, RSA keys)
- Chaos test suite: random node failures, network partitions
- Security: BGP session authentication

### Sprint 4: Advanced Features

**Goals:**
- RPKI validation (route origin verification)
- Route reflectors (for scaling >50 nodes)
- BFD for fast failure detection (<30s)
- Multi-region support (etcd replication)
- Performance tuning for 100+ nodes

**Deliverables:**
- BIRD RPKI integration with RIPE NCC validator
- Route reflector role in Ansible
- BFD configuration for all BGP sessions
- Multi-region etcd cluster (3 regions)
- Load testing: 100 nodes, convergence <2min

### Long-term Vision

- **OpenWrt integration**: Native packages for embedded routers
- **IPv6 support**: Dual-stack BGP (IPv4 + IPv6)
- **Anycast DNS**: Distributed DNS resolution
- **Metrics aggregation**: Centralized metrics from all nodes
- **Web UI**: Dashboard for node management

---

## Summary

This project is a **production-grade BGP routing framework** that combines:

1. **BIRD 3.x**: BGP routing with modern features
2. **TINC 1.0**: Mesh VPN with strong encryption
3. **etcd**: Distributed state storage with consensus
4. **Go daemon**: Custom orchestration software
5. **Docker**: Local development and testing
6. **Ansible**: Production automation
7. **Prometheus/Grafana**: Monitoring and observability

**Key Features:**
- ✅ **Automatic peer discovery**: No manual configuration
- ✅ **Self-healing**: Automatic recovery from failures
- ✅ **Scalable**: 5-node local, 50+ node production target
- ✅ **Secure**: Encrypted tunnels, authenticated BGP sessions
- ✅ **Observable**: Metrics, logs, dashboards
- ✅ **Automated**: One command to deploy

**Use Cases:**
- Mesh networks for community ISPs
- Distributed services with intelligent routing
- Research and education (learning BGP, VPNs, distributed systems)
- Resilient infrastructure for critical applications

**Current Status:**
- ✅ Sprint 1: Complete (3-node MVP)
- ✅ Sprint 2 Phase 1: Complete (5-node, tests, automation)
- 🚧 Sprint 2 Phase 2: In progress (dashboards, additional tests)
- 📅 Sprint 3: Planned (production hardening)
- 📅 Sprint 4: Planned (advanced features)

---

## Further Learning

**BGP Resources:**
- [BGP for Beginners](https://www.cisco.com/c/en/us/support/docs/ip/border-gateway-protocol-bgp/26634-bgp-toc.html)
- [BIRD Documentation](https://bird.network.cz/?get_doc)

**TINC Resources:**
- [TINC Manual](https://www.tinc-vpn.org/documentation/)
- [TINC Cookbook](https://www.tinc-vpn.org/examples/)

**etcd Resources:**
- [etcd Documentation](https://etcd.io/docs/)
- [Raft Consensus Explained](https://raft.github.io/)

**Go Programming:**
- [Go Tour](https://go.dev/tour/)
- [Effective Go](https://go.dev/doc/effective_go)

**Docker Resources:**
- [Docker Getting Started](https://docs.docker.com/get-started/)
- [Docker Compose Tutorial](https://docs.docker.com/compose/gettingstarted/)

**Ansible Resources:**
- [Ansible Getting Started](https://docs.ansible.com/ansible/latest/getting_started/index.html)
- [Ansible Best Practices](https://docs.ansible.com/ansible/latest/tips_tricks/ansible_tips_tricks.html)

---

**Generated**: November 2, 2025  
**Version**: 1.0  
**Author**: Comprehensive repository analysis for new contributors

