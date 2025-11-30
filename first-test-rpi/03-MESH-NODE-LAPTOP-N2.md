# Laptop n2 - Mesh Node Setup (Docker)

Configure Laptop n2 as a TINC mesh node (no BGP) using Docker containers.

## Device Info

- **Role**: TINC mesh node
- **Ethernet IP**: `172.30.0.101/24` (on switch network)
- **TINC IP**: `44.30.127.2/24`
- **Docker Services**: `tinc2`, `etcd1`
- **Purpose**: Participate in VPN mesh, be reachable from Mock-ISP
- **Connectivity**: Ethernet (connected to same switch as Laptop n1 and RPi)

## Network Topology

```
RPi (Mock ISP)          Laptop n1 (BGP+TINC)              Laptop n2 (TINC)
172.30.0.1              172.30.0.100 + 44.30.127.1        172.30.0.101 + 44.30.127.2
    │                        │                                │
    │◄──── Ethernet ────────►│◄────── Ethernet ──────────────►│
    │      (switch)          │        (switch)                │
    │                        │                                │
    │                        │◄──── TINC VPN Tunnel ─────────►│
    │                        │      (over 172.30.0.x)         │
```

**Physical Setup:**
- All three devices connected to the same Ethernet switch
- Switch network: 172.30.0.0/24
- TINC VPN overlay: 44.30.127.0/24

---

## Step 1: Prerequisites

### 1.1 Connect to Ethernet Switch

Connect Laptop n2 to the Ethernet switch using a cable. Configure a static IP:

```bash
# Check Ethernet interface name (usually eth0, enp0s31f6, or similar)
ip link show | grep -E "^[0-9]+:" | grep -v "lo\|docker\|br-\|veth"

# Configure static IP on the switch network
# Replace <interface> with your actual interface name (e.g., eth0, enp0s31f6)
sudo ip addr add 172.30.0.101/24 dev <interface>
sudo ip link set <interface> up

# Verify IP configuration
ip addr show <interface> | grep "inet "
# Should show: inet 172.30.0.101/24

# Test connectivity to other devices on the switch
ping -c 3 172.30.0.1    # RPi (Mock-ISP)
ping -c 3 172.30.0.100  # Laptop n1 (Border Router)
# Both should succeed
```

### 1.2 Install Docker

```bash
# Install Docker and Docker Compose
sudo apt update
sudo apt install -y docker.io docker-compose-v2

# Add user to docker group (optional)
sudo usermod -aG docker $USER
# Log out and back in

# Verify Docker
docker --version
docker compose version
```

---

## Step 2: Clone Repository

```bash
# Clone or copy repository to Laptop n2
cd ~
git clone <repository-url> BGP4mesh
cd BGP4mesh
```

---

## Step 3: Docker Compose File

The repository includes `deploy/hardware-test/docker-compose.mesh-node.yml` for the mesh node. Verify its contents:

```bash
cat deploy/hardware-test/docker-compose.mesh-node.yml
```

**Expected content:**
```yaml
version: '3.8'

services:
  tinc2:
    build: ./docker/tinc
    container_name: tinc2
    hostname: tinc2
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun
    ports:
      - "655:655/tcp"   # Meta connections (authentication)
      - "655:655/udp"   # Data transfer
    volumes:
      - ./configs/tinc:/etc/tinc:ro
      - tinc2-data:/var/run/tinc
    depends_on:
      - etcd1
    networks:
      - mesh-net
      - cluster-net
    environment:
      - TINC_NAME=node2
      - TINC_PORT=655
      - TINC_NETNAME=bgpmesh
    restart: unless-stopped

  etcd1:
    image: quay.io/coreos/etcd:v3.5.14
    container_name: etcd1
    hostname: etcd1
    command:
      - etcd
      - --name=etcd1
      - --data-dir=/etcd-data
      - --listen-client-urls=http://0.0.0.0:2379
      - --advertise-client-urls=http://etcd1:2379
      - --listen-peer-urls=http://0.0.0.0:2380
      - --initial-advertise-peer-urls=http://etcd1:2380
      - --initial-cluster=etcd1=http://etcd1:2380
      - --initial-cluster-state=new
    ports:
      - "2379:2379"
      - "2380:2380"
    volumes:
      - etcd1-data:/etcd-data
    networks:
      - cluster-net
      - mesh-net
    restart: unless-stopped

networks:
  mesh-net:
    driver: bridge
    ipam:
      config:
        - subnet: 172.22.0.0/16
  cluster-net:
    driver: bridge
    internal: true
    ipam:
      config:
        - subnet: 172.23.0.0/16

volumes:
  etcd1-data:
  tinc2-data:
```

**Key points:**
- Port 655 exposed on **both TCP and UDP** (critical for TINC authentication)
- `tinc2-data` volume persists TINC configuration and keys
- Two internal Docker networks for container communication

---

## Step 4: Deploy Services

```bash
# Deploy TINC node2
docker compose -f deploy/hardware-test/docker-compose.mesh-node.yml up -d --build

# Check status
docker ps
# Should show: tinc2, etcd1 running
```

---

## Step 5: Fix TINC Host File Address

**Critical!** The auto-generated TINC host file has `Address = tinc2` (container name) which won't resolve on separate devices. Fix it with your **Ethernet IP** on the switch network:

```bash
# View current host file
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node2

# Fix the Address line to use your Ethernet IP on the switch
docker exec tinc2 sed -i 's/Address = tinc2/Address = 172.30.0.101/' /var/run/tinc/bgpmesh/hosts/node2

# Fix the Subnet line for the 44.x network (if needed)
docker exec tinc2 sed -i 's/Subnet = 10.0.0.2\/32/Subnet = 44.30.127.2\/32/' /var/run/tinc/bgpmesh/hosts/node2

# Verify the changes
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node2
```

**Expected output:**
```
# Host configuration for node2
Address = 172.30.0.101
Port = 655
Subnet = 44.30.127.2/32

-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEA5cbOfK13bTBQi9GtLo6krkmFEuftUvY7gfU8i+AF8uvfjOSgE1D+
... (your unique key) ...
-----END RSA PUBLIC KEY-----
```

---

## Step 6: Exchange TINC Host Files

**Critical for connectivity!**

### 6.1 Get node1 host file from Laptop n1

On **Laptop n1**, get the host file:
```bash
docker exec tinc1 cat /var/run/tinc/bgpmesh/hosts/node1
```

**Important**: The Address should be Laptop n1's **macvlan IP** (`172.30.0.100`), which is its IP on the switch network.

### 6.2 Create node1 host file on Laptop n2

```bash
# Create node1 host file on Laptop n2
docker exec tinc2 sh -c 'cat > /var/run/tinc/bgpmesh/hosts/node1' << 'EOF'
# Host configuration for node1
Address = 172.30.0.100
Port = 655
Subnet = 44.30.127.1/32

-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEApfuQcJQ2gdEd2WUU1Aav4b0UoWNwtxgWlkxzb6xgPxjyECwPPRBA
WLuLbHpPWrIRr2txaIEfoukexh4eGirFnvo1S8vdX9S7xQsUvK0h/z20Zdv6d7ny
yXv75Ponb82kj/ZqjuZUZ6b8SSWiInD0OfZJNGxGQK/UyZ6ZVHL/op8w0QZi+Fub
WNh8yCzP7EAj1UNRzbkstiiKQrvTllwRJh6u9JMWhZk/ommo7KYVMu0iaGNf0DZ3
LkAA0KKBKqLgGcS5hJu/4lvq89xaX0mqIu48qouUhBq5vDaeO81c4LbgFNXM71DR
arbrAh7EodXw41sYZgBqjytGOx0U+W1guQIDAQAB
-----END RSA PUBLIC KEY-----
EOF
```

**Note:** Replace the RSA key with the actual key from Laptop n1's host file.

### 6.3 Send node2 host file to Laptop n1

```bash
# Display host file for Laptop n1 (with corrected Address)
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node2
# Copy this entire output and send to Laptop n1
```

On **Laptop n1**, add node2's host file:
```bash
# Run on Laptop n1:
docker exec tinc1 sh -c 'cat > /var/run/tinc/bgpmesh/hosts/node2' << 'EOF'
# Paste node2 content here (with Address = 172.30.0.101)
EOF

# Restart TINC on Laptop n1 to pick up new host file
docker compose -f deploy/hardware-test/docker-compose.border-router.yml restart tinc1
```

### 6.4 Verify both host files exist with correct Address

```bash
docker exec tinc2 ls -la /var/run/tinc/bgpmesh/hosts/
# Should show: node1, node2

# Verify Address lines are Ethernet IPs (not container names)
docker exec tinc2 grep "Address" /var/run/tinc/bgpmesh/hosts/*
# node1: Address = 172.30.0.100 (Laptop n1 macvlan IP)
# node2: Address = 172.30.0.101 (Laptop n2 Ethernet IP)
```

---

## Step 7: Configure TINC to Connect to node1

The template doesn't include `ConnectTo` by default. Add it:

```bash
# Check current config
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc.conf

# Add ConnectTo directive
docker exec tinc2 sh -c 'echo "ConnectTo = node1" >> /var/run/tinc/bgpmesh/tinc.conf'

# Verify the config
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc.conf
```

**Expected tinc.conf:**
```
# TINC 1.0 Configuration
# Generated from template

Name = node2
Mode = switch
Cipher = aes-256-cbc
Digest = sha256
Port = 655
Interface = tinc0

# Compression (optional, can add overhead)
# Compression = 9

# Forwarding
# DeviceType = tun
ConnectTo = node1
```

```bash
# Restart TINC to apply changes
docker compose -f deploy/hardware-test/docker-compose.mesh-node.yml restart tinc2
```

---

## Step 8: Verify Connectivity

### 8.1 Check TINC Interface

```bash
# Interface should be up
docker exec tinc2 ip addr show tinc0
# Expected: 44.30.127.2/24 UP

# Check logs for connection to node1
docker exec tinc2 tail -30 /var/run/tinc/bgpmesh/tinc.log | grep -E "PING|PONG|node1|Connected"
# Should show PING/PONG exchanges with node1
```

### 8.2 Test Ethernet Connectivity to Laptop n1

```bash
# Verify Ethernet path works (from host)
ping -c 3 172.30.0.100
# Should succeed
```

### 8.3 Ping Laptop n1 via TINC

```bash
# Test TINC mesh connectivity (from inside container)
docker exec tinc2 ping -c 5 44.30.127.1
# Should succeed
```

### 8.4 Check Routing Table

```bash
# View routes inside container
docker exec tinc2 ip route
# Should show: 44.30.127.0/24 dev tinc0 proto kernel scope link src 44.30.127.2
```

---

## Step 9: Configure Return Route for Mock-ISP

For Mock-ISP to successfully ping Laptop n2, ensure routing back to ISP network:

```bash
# Add route to ISP (172.30.0.1) via Laptop n1's TINC address
docker exec tinc2 ip route add 172.30.0.1 via 44.30.127.1 dev tinc0

# Verify the route was added
docker exec tinc2 ip route | grep 172.30
# Should show: 172.30.0.1 via 44.30.127.1 dev tinc0
```

**Alternative:** Add route to entire ISP network:
```bash
docker exec tinc2 ip route add 172.30.0.0/24 via 44.30.127.1 dev tinc0
```

**Note**: These routes are temporary and will be lost on container restart. For persistence:
1. Add to a startup script
2. Modify the `tinc-up` script
3. Use Docker entrypoint modification

---

## Step 10: Test from Mock-ISP

Once all devices are configured:

### On Mock-ISP (Raspberry Pi):

```bash
# Ping Laptop n2 via TINC
ping -c 5 44.30.127.2
# Should succeed ✅ Goal achieved!

# Trace route
traceroute 44.30.127.2
# Should show: RPi → Laptop n1 (172.30.0.100) → Laptop n2 (44.30.127.2)
```

### On Laptop n2 (verify responses):

```bash
# Monitor ICMP traffic
docker exec tinc2 tcpdump -i tinc0 icmp
# Should see echo requests from Mock-ISP and echo replies
```

---

## Troubleshooting

### TINC Not Starting

```bash
# Check logs
docker logs tinc2 | tail -50

# Check config syntax
docker exec tinc2 tincd -n bgpmesh -D -d5
# Watch for errors (Ctrl+C to exit)

# Check host files
docker exec tinc2 ls -la /var/run/tinc/bgpmesh/hosts/
# Must have both node1 and node2

# Check TUN device
docker exec tinc2 ls -l /dev/net/tun
# Should exist
```

### Connection Timeout During Authentication

**Symptom:**
```
Timeout from node1 (172.30.0.100 port 655) during authentication
```

**Root Cause:** Only UDP port 655 exposed, but TINC needs TCP for authentication.

**Solution:** Ensure docker-compose.mesh-node.yml has both TCP and UDP:
```yaml
ports:
  - "655:655/tcp"   # Meta connections (authentication)
  - "655:655/udp"   # Data transfer
```

### Ping from Laptop n1 Works, but Mock-ISP Ping Fails

```bash
# Check routing on Laptop n2
docker exec tinc2 ip route
# Must have route back to 172.30.0.1 via 44.30.127.1

# Add route if missing
docker exec tinc2 ip route add 172.30.0.1 via 44.30.127.1 dev tinc0

# Test again from Mock-ISP
```

### TINC Interface Not Coming Up

```bash
# Check tinc-up permissions
docker exec tinc2 ls -l /var/run/tinc/bgpmesh/tinc-up
# Should be executable

# Check tinc-up content
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc-up
# Should configure 44.30.127.2/24

# Restart TINC
docker compose -f deploy/hardware-test/docker-compose.mesh-node.yml restart tinc2
```

### No Connection to node1

```bash
# Check node1 host file exists and has correct Address line
docker exec tinc2 cat /var/run/tinc/bgpmesh/hosts/node1
# Must have: Address = 172.30.0.100 (Laptop n1's macvlan IP, not "tinc1"!)

# Check tinc.conf has ConnectTo
docker exec tinc2 cat /var/run/tinc/bgpmesh/tinc.conf | grep ConnectTo
# Should show: ConnectTo = node1

# Manual connection attempt
docker exec tinc2 tinc -n bgpmesh connect node1

# Check network connectivity to Laptop n1 via Ethernet
ping 172.30.0.100  # Test Ethernet connectivity to Laptop n1

# Check if port 655 is reachable on Laptop n1
nc -zv 172.30.0.100 655
# Should show connection succeeded
```

### TINC Host Files Have Wrong Address (Container Names)

If host files have `Address = tinc1` or `Address = tinc2` instead of IPs:

```bash
# Check Address lines
docker exec tinc2 grep "Address" /var/run/tinc/bgpmesh/hosts/*

# If node1 has "Address = tinc1", fix it:
docker exec tinc2 sed -i 's/Address = tinc1/Address = 172.30.0.100/' /var/run/tinc/bgpmesh/hosts/node1

# If node2 has "Address = tinc2", fix it:
docker exec tinc2 sed -i 's/Address = tinc2/Address = 172.30.0.101/' /var/run/tinc/bgpmesh/hosts/node2

# Also check Subnet lines - should be 44.x network, not 10.x
docker exec tinc2 grep "Subnet" /var/run/tinc/bgpmesh/hosts/*
# node1: Subnet = 44.30.127.1/32
# node2: Subnet = 44.30.127.2/32

# Restart TINC
docker compose -f deploy/hardware-test/docker-compose.mesh-node.yml restart tinc2
```

### etcd Connection Issues

```bash
# Check etcd is running
docker ps | grep etcd1

# Check etcd logs
docker logs etcd1

# Verify etcd connectivity from tinc2
docker exec tinc2 etcdctl --endpoints=http://etcd1:2379 endpoint health
# Should show healthy
```

---

## Configuration Files Summary

### Files from Repository:
- **Docker Compose**: `deploy/hardware-test/docker-compose.mesh-node.yml`
- **TINC templates**: `configs/tinc/tinc.conf.j2`, `configs/tinc/tinc-up.j2`, `configs/tinc/tinc-down.j2`
- **Docker image**: `docker/tinc/Dockerfile`
- **Entrypoint**: `docker/tinc/entrypoint.sh`

### Generated Files in Container (`/var/run/tinc/bgpmesh/`):
- `tinc.conf` - Main TINC configuration
- `tinc-up` - Interface up script (configures 44.30.127.2/24)
- `tinc-down` - Interface down script
- `rsa_key.priv` - Private RSA key
- `rsa_key.pub` - Public RSA key
- `hosts/node1` - Laptop n1 host file (with public key)
- `hosts/node2` - This node's host file (with public key)
- `tinc.log` - TINC daemon log

---

## Final Configuration State

### tinc.conf
```
# TINC 1.0 Configuration
# Generated from template

Name = node2
Mode = switch
Cipher = aes-256-cbc
Digest = sha256
Port = 655
Interface = tinc0

# Compression (optional, can add overhead)
# Compression = 9

# Forwarding
# DeviceType = tun
ConnectTo = node1
```

### hosts/node1
```
# Host configuration for node1
Address = 172.30.0.100
Port = 655
Subnet = 44.30.127.1/32

-----BEGIN RSA PUBLIC KEY-----
... (Laptop n1's public key) ...
-----END RSA PUBLIC KEY-----
```

### hosts/node2
```
# Host configuration for node2
Address = 172.30.0.101
Port = 655
Subnet = 44.30.127.2/32

-----BEGIN RSA PUBLIC KEY-----
... (This node's public key) ...
-----END RSA PUBLIC KEY-----
```

### Container Network Interfaces
```
eth0: 172.23.0.3/16  (cluster-net - internal Docker network)
eth1: 172.22.0.3/16  (mesh-net - Docker network with gateway)
tinc0: 44.30.127.2/24 (TINC VPN interface)
```

### Container Routes
```
default via 172.22.0.1 dev eth1
44.30.127.0/24 dev tinc0 proto kernel scope link src 44.30.127.2
172.22.0.0/16 dev eth1 proto kernel scope link src 172.22.0.3
172.23.0.0/16 dev eth0 proto kernel scope link src 172.23.0.3
172.30.0.1 via 44.30.127.1 dev tinc0  # Return route to ISP
```

---

## Verification Checklist

- [ ] Connected to Ethernet switch with IP 172.30.0.101
- [ ] Can ping RPi (172.30.0.1) and Laptop n1 (172.30.0.100) from host
- [ ] Docker services running (`docker ps | grep -E "tinc2|etcd1"`)
- [ ] tinc0 interface UP with `44.30.127.2/24` (`docker exec tinc2 ip addr show tinc0`)
- [ ] Host files have correct Addresses (172.30.0.x, not container names)
- [ ] `ConnectTo = node1` in tinc.conf
- [ ] Can ping Laptop n1 TINC IP (`docker exec tinc2 ping 44.30.127.1`)
- [ ] Return route to ISP exists (`docker exec tinc2 ip route | grep 172.30`)
- [ ] **Mock-ISP can ping this device (44.30.127.2)** ✅

---

## Final Test

From **Raspberry Pi**:
```bash
ping -c 10 44.30.127.2
# Success! Goal achieved!
```

This proves:
- BGP routing works (RPi → Laptop n1)
- TINC mesh works (Laptop n1 → Laptop n2)
- Full end-to-end connectivity established

---

## Packet Flow: Mock-ISP → Laptop n2

1. **RPi (172.30.0.1)** sends ICMP to 44.30.127.2
2. **RPi kernel route:** 44.30.127.0/24 via 172.30.0.100 → forwards to Laptop n1
3. **Laptop n1 (172.30.0.100)** receives on macvlan interface (eth1)
4. **IP forwarding** enabled in tinc1 container, looks up route: 44.30.127.0/24 dev tinc0
5. **TINC** encrypts and sends via UDP to 172.30.0.101:655
6. **Laptop n2 host** receives on Ethernet, Docker NAT forwards to tinc2 container
7. **TINC** decrypts and delivers to tinc0 interface
8. **Destination:** 44.30.127.2 reached
9. **Return path:** Reply goes via 172.30.0.1 route → 44.30.127.1 → TINC tunnel → Laptop n1 → Ethernet → RPi
